package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbapikey "github.com/Wei-Shaw/sub2api/ent/apikey"
	dbbinding "github.com/Wei-Shaw/sub2api/ent/apikeysubpoolbinding"
	dbsubpool "github.com/Wei-Shaw/sub2api/ent/subpool"
	dbsubpoolaccount "github.com/Wei-Shaw/sub2api/ent/subpoolaccount"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subPoolRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

// NewSubPoolRepository wires the sub-pool persistence layer. sqlDB is used for
// scheduler outbox notifications so that membership changes invalidate the
// scheduler snapshot the same way account_groups changes do.
func NewSubPoolRepository(client *dbent.Client, sqlDB *sql.DB) service.SubPoolRepository {
	return &subPoolRepository{client: client, sql: sqlDB}
}

func subPoolEntityToService(row *dbent.SubPool) *service.SubPool {
	if row == nil {
		return nil
	}
	return &service.SubPool{
		ID:            row.ID,
		GroupID:       row.GroupID,
		Name:          row.Name,
		Description:   row.Description,
		Kind:          row.Kind,
		Status:        row.Status,
		KeySoftLimit:  row.KeySoftLimit,
		CoolingUntil:  row.CoolingUntil,
		CoolingReason: row.CoolingReason,
		SortOrder:     row.SortOrder,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func subPoolBindingEntityToService(row *dbent.APIKeySubPoolBinding) *service.SubPoolBinding {
	if row == nil {
		return nil
	}
	return &service.SubPoolBinding{
		ID:        row.ID,
		APIKeyID:  row.APIKeyID,
		SubPoolID: row.SubPoolID,
		GroupID:   row.GroupID,
		BoundAt:   row.BoundAt,
		UnboundAt: row.UnboundAt,
		Reason:    row.Reason,
		Operator:  row.Operator,
		Note:      row.Note,
	}
}

func (r *subPoolRepository) Create(ctx context.Context, pool *service.SubPool) error {
	if pool == nil {
		return errors.New("sub-pool is nil")
	}
	builder := r.client.SubPool.Create().
		SetGroupID(pool.GroupID).
		SetName(pool.Name).
		SetNillableDescription(pool.Description).
		SetKind(pool.Kind).
		SetStatus(pool.Status).
		SetKeySoftLimit(pool.KeySoftLimit).
		SetSortOrder(pool.SortOrder).
		SetNillableCoolingUntil(pool.CoolingUntil).
		SetNillableCoolingReason(pool.CoolingReason)
	row, err := builder.Save(ctx)
	if err != nil {
		if dbent.IsConstraintError(err) {
			return service.ErrSubPoolExists
		}
		return err
	}
	pool.ID = row.ID
	pool.CreatedAt = row.CreatedAt
	pool.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *subPoolRepository) GetByID(ctx context.Context, id int64) (*service.SubPool, error) {
	row, err := r.client.SubPool.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrSubPoolNotFound
		}
		return nil, err
	}
	pool := subPoolEntityToService(row)
	accountIDs, err := r.ListAccountIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	pool.AccountIDs = accountIDs
	counts, err := r.countKeysByPool(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	pool.BoundKeys = counts[id]
	return pool, nil
}

func (r *subPoolRepository) Update(ctx context.Context, pool *service.SubPool) error {
	if pool == nil {
		return errors.New("sub-pool is nil")
	}
	update := r.client.SubPool.UpdateOneID(pool.ID).
		SetName(pool.Name).
		SetKind(pool.Kind).
		SetStatus(pool.Status).
		SetKeySoftLimit(pool.KeySoftLimit).
		SetSortOrder(pool.SortOrder).
		SetUpdatedAt(timezone.Now())
	if pool.Description != nil {
		update = update.SetDescription(*pool.Description)
	} else {
		update = update.ClearDescription()
	}
	if pool.CoolingUntil != nil {
		update = update.SetCoolingUntil(*pool.CoolingUntil)
	} else {
		update = update.ClearCoolingUntil()
	}
	if pool.CoolingReason != nil {
		update = update.SetCoolingReason(*pool.CoolingReason)
	} else {
		update = update.ClearCoolingReason()
	}
	row, err := update.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrSubPoolNotFound
		}
		if dbent.IsConstraintError(err) {
			return service.ErrSubPoolExists
		}
		return err
	}
	pool.UpdatedAt = row.UpdatedAt
	r.notifyGroupChanged(ctx, pool.GroupID)
	return nil
}

// Delete soft-deletes the pool and detaches its accounts and keys. Keys fall
// back to whole-group scheduling rather than being stranded on a dead pool.
func (r *subPoolRepository) Delete(ctx context.Context, id int64) error {
	pool, err := r.client.SubPool.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrSubPoolNotFound
		}
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		tx = nil
		txClient = r.client
	}

	keyIDs, err := txClient.APIKey.Query().
		Where(dbapikey.SubPoolIDEQ(id)).
		IDs(ctx)
	if err != nil {
		return err
	}
	if len(keyIDs) > 0 {
		if _, err := txClient.APIKey.Update().
			Where(dbapikey.IDIn(keyIDs...)).
			ClearSubPoolID().
			Save(ctx); err != nil {
			return err
		}
		if err := closeOpenSubPoolBindings(ctx, txClient, keyIDs); err != nil {
			return err
		}
	}
	if _, err := txClient.SubPoolAccount.Delete().
		Where(dbsubpoolaccount.SubPoolIDEQ(id)).
		Exec(ctx); err != nil {
		return err
	}
	if err := txClient.SubPool.DeleteOneID(id).Exec(ctx); err != nil {
		return err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	r.notifyGroupChanged(ctx, pool.GroupID)
	return nil
}

func (r *subPoolRepository) ListByGroup(ctx context.Context, groupID int64) ([]service.SubPool, error) {
	rows, err := r.client.SubPool.Query().
		Where(dbsubpool.GroupIDEQ(groupID)).
		Order(dbent.Asc(dbsubpool.FieldSortOrder), dbent.Asc(dbsubpool.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	pools := make([]service.SubPool, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		pools = append(pools, *subPoolEntityToService(row))
		ids = append(ids, row.ID)
	}
	if len(ids) == 0 {
		return pools, nil
	}

	members, err := r.client.SubPoolAccount.Query().
		Where(dbsubpoolaccount.SubPoolIDIn(ids...)).
		Order(dbent.Asc(dbsubpoolaccount.FieldAccountID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	accountsByPool := make(map[int64][]int64, len(ids))
	for _, m := range members {
		accountsByPool[m.SubPoolID] = append(accountsByPool[m.SubPoolID], m.AccountID)
	}
	counts, err := r.countKeysByPool(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range pools {
		pools[i].AccountIDs = accountsByPool[pools[i].ID]
		pools[i].BoundKeys = counts[pools[i].ID]
	}
	return pools, nil
}

const subPoolGroupKeyListLimit = 500

func (r *subPoolRepository) ListGroupKeys(ctx context.Context, groupID int64) ([]service.SubPoolGroupKey, error) {
	rows, err := r.client.APIKey.Query().
		Where(dbapikey.GroupIDEQ(groupID), dbapikey.DeletedAtIsNil()).
		WithUser(func(q *dbent.UserQuery) {
			q.Select(dbuser.FieldID, dbuser.FieldEmail, dbuser.FieldUsername)
		}).
		Order(dbent.Asc(dbapikey.FieldUserID), dbent.Asc(dbapikey.FieldID)).
		Limit(subPoolGroupKeyListLimit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	defaults, err := r.userDefaultByUser(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]service.SubPoolGroupKey, 0, len(rows))
	for _, row := range rows {
		item := service.SubPoolGroupKey{
			APIKeyID:  row.ID,
			Name:      row.Name,
			UserID:    row.UserID,
			Status:    row.Status,
			SubPoolID: row.SubPoolID,
		}
		if u := row.Edges.User; u != nil {
			item.UserEmail = u.Email
			item.UserUsername = u.Username
		}
		if poolID, ok := defaults[row.UserID]; ok {
			id := poolID
			item.UserDefaultSubPoolID = &id
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *subPoolRepository) userDefaultByUser(ctx context.Context, groupID int64) (map[int64]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id, sub_pool_id
		FROM user_sub_pool_defaults
		WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]int64)
	for rows.Next() {
		var userID, poolID int64
		if err := rows.Scan(&userID, &poolID); err != nil {
			return nil, err
		}
		out[userID] = poolID
	}
	return out, rows.Err()
}

func (r *subPoolRepository) GetGroupDefaultPool(ctx context.Context, groupID int64) (*int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT sub_pool_id FROM group_sub_pool_defaults WHERE group_id = $1`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var id int64
	if err := rows.Scan(&id); err != nil {
		return nil, err
	}
	return &id, rows.Err()
}

func (r *subPoolRepository) SetGroupDefaultPool(ctx context.Context, groupID int64, subPoolID *int64) error {
	if subPoolID == nil {
		_, err := r.sql.ExecContext(ctx, `DELETE FROM group_sub_pool_defaults WHERE group_id = $1`, groupID)
		return err
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO group_sub_pool_defaults (group_id, sub_pool_id, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (group_id) DO UPDATE SET sub_pool_id = EXCLUDED.sub_pool_id, updated_at = NOW()`,
		groupID, *subPoolID)
	return err
}

func (r *subPoolRepository) GetUserDefaultPool(ctx context.Context, userID, groupID int64) (*int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT sub_pool_id FROM user_sub_pool_defaults WHERE user_id = $1 AND group_id = $2`,
		userID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var id int64
	if err := rows.Scan(&id); err != nil {
		return nil, err
	}
	return &id, rows.Err()
}

func (r *subPoolRepository) ListUserDefaults(ctx context.Context, groupID int64) ([]service.UserSubPoolDefault, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT d.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
		       d.group_id, d.sub_pool_id, d.operator, d.note, d.updated_at
		FROM user_sub_pool_defaults d
		JOIN users u ON u.id = d.user_id
		WHERE d.group_id = $1
		ORDER BY u.email, d.user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.UserSubPoolDefault, 0)
	for rows.Next() {
		var row service.UserSubPoolDefault
		if err := rows.Scan(
			&row.UserID, &row.UserEmail, &row.UserUsername,
			&row.GroupID, &row.SubPoolID, &row.Operator, &row.Note, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *subPoolRepository) SetUserDefaultPool(ctx context.Context, in service.UserSubPoolDefault) error {
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO user_sub_pool_defaults (user_id, group_id, sub_pool_id, operator, note, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (user_id, group_id) DO UPDATE
		SET sub_pool_id = EXCLUDED.sub_pool_id,
		    operator = EXCLUDED.operator,
		    note = EXCLUDED.note,
		    updated_at = NOW()`,
		in.UserID, in.GroupID, in.SubPoolID, in.Operator, in.Note)
	return err
}

func (r *subPoolRepository) ClearUserDefaultPool(ctx context.Context, userID, groupID int64) error {
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_sub_pool_defaults WHERE user_id = $1 AND group_id = $2`, userID, groupID)
	return err
}

func (r *subPoolRepository) ListKeyIDsByUserGroup(ctx context.Context, userID, groupID int64) ([]int64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id FROM api_keys
		WHERE user_id = $1 AND group_id = $2 AND deleted_at IS NULL
		ORDER BY id`, userID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *subPoolRepository) ExistsByName(ctx context.Context, groupID int64, name string, excludeID int64) (bool, error) {
	query := r.client.SubPool.Query().
		Where(dbsubpool.GroupIDEQ(groupID), dbsubpool.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(dbsubpool.IDNEQ(excludeID))
	}
	return query.Exist(ctx)
}

func (r *subPoolRepository) SetAccounts(ctx context.Context, subPoolID int64, accountIDs []int64) error {
	pool, err := r.client.SubPool.Get(ctx, subPoolID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrSubPoolNotFound
		}
		return err
	}

	previous, err := r.ListAccountIDs(ctx, subPoolID)
	if err != nil {
		return err
	}

	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		tx = nil
		txClient = r.client
	}

	if _, err := txClient.SubPoolAccount.Delete().
		Where(dbsubpoolaccount.SubPoolIDEQ(subPoolID)).
		Exec(ctx); err != nil {
		return err
	}
	if len(accountIDs) > 0 {
		builders := make([]*dbent.SubPoolAccountCreate, 0, len(accountIDs))
		for _, accountID := range accountIDs {
			builders = append(builders, txClient.SubPoolAccount.Create().
				SetSubPoolID(subPoolID).
				SetAccountID(accountID).
				SetGroupID(pool.GroupID).
				SetRole(domain.SubPoolAccountRolePrimary))
		}
		if _, err := txClient.SubPoolAccount.CreateBulk(builders...).Save(ctx); err != nil {
			if dbent.IsConstraintError(err) {
				return service.ErrSubPoolAccountTaken
			}
			return err
		}
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// Scheduling candidates changed for every account that entered or left.
	for _, accountID := range mergeGroupIDs(previous, accountIDs) {
		id := accountID
		if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountGroupsChanged, &id, nil, nil); err != nil {
			logger.LegacyPrintf("repository.subpool", "[SchedulerOutbox] enqueue sub-pool accounts failed: account=%d err=%v", id, err)
		}
	}
	r.notifyGroupChanged(ctx, pool.GroupID)
	return nil
}

func (r *subPoolRepository) ListAccountIDs(ctx context.Context, subPoolID int64) ([]int64, error) {
	rows, err := r.client.SubPoolAccount.Query().
		Where(dbsubpoolaccount.SubPoolIDEQ(subPoolID)).
		Order(dbent.Asc(dbsubpoolaccount.FieldAccountID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AccountID)
	}
	return ids, nil
}

func (r *subPoolRepository) BindKey(ctx context.Context, in service.SubPoolBindInput) error {
	if in.APIKeyID <= 0 || in.SubPoolID <= 0 {
		return errors.New("api key id and sub-pool id are required")
	}
	if in.Reason == "" {
		return errors.New("bind reason is required")
	}
	if in.Operator == "" {
		in.Operator = domain.SubPoolBindOperatorSystem
	}

	current, err := r.client.APIKey.Get(ctx, in.APIKeyID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrAPIKeyNotFound
		}
		return err
	}
	if current.SubPoolID != nil && *current.SubPoolID == in.SubPoolID {
		return nil
	}

	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		tx = nil
		txClient = r.client
	}

	if err := closeOpenSubPoolBindings(ctx, txClient, []int64{in.APIKeyID}); err != nil {
		return err
	}
	if _, err := txClient.APIKey.UpdateOneID(in.APIKeyID).
		SetSubPoolID(in.SubPoolID).
		Save(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrAPIKeyNotFound
		}
		return err
	}
	if _, err := txClient.APIKeySubPoolBinding.Create().
		SetAPIKeyID(in.APIKeyID).
		SetSubPoolID(in.SubPoolID).
		SetGroupID(in.GroupID).
		SetBoundAt(timezone.Now()).
		SetReason(in.Reason).
		SetOperator(in.Operator).
		SetNillableNote(in.Note).
		Save(ctx); err != nil {
		return err
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (r *subPoolRepository) UnbindKey(ctx context.Context, apiKeyID int64, reason, operator string) error {
	if apiKeyID <= 0 {
		return errors.New("api key id is required")
	}
	_ = reason
	_ = operator

	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	var txClient *dbent.Client
	if err == nil {
		defer func() { _ = tx.Rollback() }()
		txClient = tx.Client()
	} else {
		tx = nil
		txClient = r.client
	}

	if _, err := txClient.APIKey.UpdateOneID(apiKeyID).
		ClearSubPoolID().
		Save(ctx); err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrAPIKeyNotFound
		}
		return err
	}
	if err := closeOpenSubPoolBindings(ctx, txClient, []int64{apiKeyID}); err != nil {
		return err
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (r *subPoolRepository) ListKeyIDs(ctx context.Context, subPoolID int64) ([]int64, error) {
	return r.client.APIKey.Query().
		Where(dbapikey.SubPoolIDEQ(subPoolID)).
		Order(dbent.Asc(dbapikey.FieldID)).
		IDs(ctx)
}

func (r *subPoolRepository) ListBindingHistory(ctx context.Context, apiKeyID int64, limit int) ([]service.SubPoolBinding, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.client.APIKeySubPoolBinding.Query().
		Where(dbbinding.APIKeyIDEQ(apiKeyID)).
		Order(dbent.Desc(dbbinding.FieldBoundAt), dbent.Desc(dbbinding.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.SubPoolBinding, 0, len(rows))
	for _, row := range rows {
		out = append(out, *subPoolBindingEntityToService(row))
	}
	return out, nil
}

func (r *subPoolRepository) GetOpenBinding(ctx context.Context, apiKeyID int64) (*service.SubPoolBinding, error) {
	row, err := r.client.APIKeySubPoolBinding.Query().
		Where(dbbinding.APIKeyIDEQ(apiKeyID), dbbinding.UnboundAtIsNil()).
		Order(dbent.Desc(dbbinding.FieldBoundAt)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return subPoolBindingEntityToService(row), nil
}

// ListProbationCandidates finds keys whose probe-pool stay is long enough to
// qualify for graduation. It reads bound_at from the open binding row rather
// than from the key row, so a key that was moved between probe pools restarts
// its clock — which is the point of a move.
func (r *subPoolRepository) ListProbationCandidates(ctx context.Context, boundBefore time.Time, limit int) ([]service.SubPoolProbationCandidate, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
		SELECT b.api_key_id, b.group_id, b.sub_pool_id, b.bound_at
		FROM api_key_sub_pool_bindings b
		JOIN api_keys k ON k.id = b.api_key_id AND k.sub_pool_id = b.sub_pool_id
		JOIN sub_pools p ON p.id = b.sub_pool_id AND p.deleted_at IS NULL
		JOIN groups g ON g.id = b.group_id
		WHERE b.unbound_at IS NULL
		  AND b.bound_at <= $1
		  AND p.kind = $2
		  AND g.sub_pool_enabled
		  AND k.status = $3
		  AND NOT EXISTS (
		      SELECT 1 FROM user_sub_pool_defaults d
		      WHERE d.user_id = k.user_id AND d.group_id = b.group_id
		  )
		ORDER BY b.bound_at
		LIMIT $4`

	rows, err := r.sql.QueryContext(ctx, query, boundBefore, domain.SubPoolKindProbe, domain.StatusActive, limit)
	if err != nil {
		return nil, fmt.Errorf("query sub-pool probation candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.SubPoolProbationCandidate, 0, limit)
	for rows.Next() {
		var c service.SubPoolProbationCandidate
		if err := rows.Scan(&c.APIKeyID, &c.GroupID, &c.SubPoolID, &c.BoundAt); err != nil {
			return nil, fmt.Errorf("scan sub-pool probation candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetSchedulingState is the hot-path read: status plus account allowlist, with
// none of the key counting that GetByID does.
func (r *subPoolRepository) GetSchedulingState(ctx context.Context, subPoolID int64) (*service.SubPoolSchedulingState, error) {
	row, err := r.client.SubPool.Query().
		Where(dbsubpool.IDEQ(subPoolID)).
		Select(dbsubpool.FieldStatus).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrSubPoolNotFound
		}
		return nil, err
	}
	accountIDs, err := r.ListAccountIDs(ctx, subPoolID)
	if err != nil {
		return nil, err
	}
	return &service.SubPoolSchedulingState{Status: row.Status, AccountIDs: accountIDs}, nil
}

// ListPoolsInEnabledGroups feeds the cooling sweep. Groups without sub-pool
// scheduling are skipped so the job stays proportional to the feature's usage
// rather than to the total number of groups.
func (r *subPoolRepository) ListPoolsInEnabledGroups(ctx context.Context) ([]service.SubPool, error) {
	const query = `
		SELECT p.id, p.group_id, p.name, p.kind, p.status, p.key_soft_limit,
		       p.cooling_until, p.cooling_reason, p.sort_order
		FROM sub_pools p
		JOIN groups g ON g.id = p.group_id
		WHERE p.deleted_at IS NULL AND g.sub_pool_enabled
		ORDER BY p.group_id, p.sort_order, p.id`

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query sub-pools in enabled groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	pools := make([]service.SubPool, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		var pool service.SubPool
		if err := rows.Scan(&pool.ID, &pool.GroupID, &pool.Name, &pool.Kind, &pool.Status,
			&pool.KeySoftLimit, &pool.CoolingUntil, &pool.CoolingReason, &pool.SortOrder); err != nil {
			return nil, fmt.Errorf("scan sub-pool in enabled group: %w", err)
		}
		pools = append(pools, pool)
		ids = append(ids, pool.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return pools, nil
	}

	members, err := r.client.SubPoolAccount.Query().
		Where(dbsubpoolaccount.SubPoolIDIn(ids...)).
		Order(dbent.Asc(dbsubpoolaccount.FieldAccountID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	accountsByPool := make(map[int64][]int64, len(ids))
	for _, m := range members {
		accountsByPool[m.SubPoolID] = append(accountsByPool[m.SubPoolID], m.AccountID)
	}
	counts, err := r.countKeysByPool(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range pools {
		pools[i].AccountIDs = accountsByPool[pools[i].ID]
		pools[i].BoundKeys = counts[pools[i].ID]
	}
	return pools, nil
}

// CountAutoMigrationsSince counts system-initiated cooling migrations of a key.
// Admin moves are excluded on purpose: the debounce protects users from pools
// flapping, it must not stop an operator from acting.
func (r *subPoolRepository) CountAutoMigrationsSince(ctx context.Context, apiKeyID int64, since time.Time) (int, error) {
	return r.client.APIKeySubPoolBinding.Query().
		Where(
			dbbinding.APIKeyIDEQ(apiKeyID),
			dbbinding.ReasonEQ(domain.SubPoolBindReasonCoolingMigration),
			dbbinding.OperatorEQ(domain.SubPoolBindOperatorSystem),
			dbbinding.BoundAtGTE(since),
		).
		Count(ctx)
}

func (r *subPoolRepository) countKeysByPool(ctx context.Context, subPoolIDs []int64) (map[int64]int, error) {
	counts := make(map[int64]int, len(subPoolIDs))
	if len(subPoolIDs) == 0 {
		return counts, nil
	}
	rows, err := r.client.APIKey.Query().
		Where(dbapikey.SubPoolIDIn(subPoolIDs...)).
		Select(dbapikey.FieldSubPoolID).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.SubPoolID == nil {
			continue
		}
		counts[*row.SubPoolID]++
	}
	return counts, nil
}

// notifyGroupChanged invalidates the scheduler snapshot for the group so that
// the next selection sees the new pool membership.
func (r *subPoolRepository) notifyGroupChanged(ctx context.Context, groupID int64) {
	id := groupID
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventGroupChanged, nil, &id, nil); err != nil {
		logger.LegacyPrintf("repository.subpool", "[SchedulerOutbox] enqueue sub-pool group change failed: group=%d err=%v", groupID, err)
	}
}

func closeOpenSubPoolBindings(ctx context.Context, client *dbent.Client, apiKeyIDs []int64) error {
	if len(apiKeyIDs) == 0 {
		return nil
	}
	if _, err := client.APIKeySubPoolBinding.Update().
		Where(dbbinding.APIKeyIDIn(apiKeyIDs...), dbbinding.UnboundAtIsNil()).
		SetUnboundAt(timezone.Now()).
		Save(ctx); err != nil {
		return fmt.Errorf("close open sub-pool bindings: %w", err)
	}
	return nil
}
