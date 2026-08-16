package domain

// UpstreamRateVersionSource identifies where an account's upstream billing
// multiplier came from.
type UpstreamRateVersionSource string

const (
	UpstreamRateSourceDefault       UpstreamRateVersionSource = "default"
	UpstreamRateSourceManual        UpstreamRateVersionSource = "manual"
	UpstreamRateSourceUpstreamProbe UpstreamRateVersionSource = "upstream_probe"
)

func (s UpstreamRateVersionSource) Valid() bool {
	switch s {
	case UpstreamRateSourceDefault, UpstreamRateSourceManual, UpstreamRateSourceUpstreamProbe:
		return true
	default:
		return false
	}
}

// UpstreamRateVersionChangeReason explains why a new version became active.
type UpstreamRateVersionChangeReason string

const (
	UpstreamRateChangeAccountCreated UpstreamRateVersionChangeReason = "account_created"
	UpstreamRateChangeManualUpdate   UpstreamRateVersionChangeReason = "manual_update"
	UpstreamRateChangeProbeChanged   UpstreamRateVersionChangeReason = "probe_changed"
	UpstreamRateChangeProbeTakeover  UpstreamRateVersionChangeReason = "probe_takeover"
	UpstreamRateChangeManualTakeover UpstreamRateVersionChangeReason = "manual_takeover"
)
