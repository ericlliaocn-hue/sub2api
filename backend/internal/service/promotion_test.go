package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePromotionChannelInput(t *testing.T) {
	rate := 12.5
	input := PromotionChannelInput{
		Code:           "  forum-cn_01 ",
		Name:           "  中文论坛  ",
		ChannelType:    " 论坛 ",
		CommissionRate: &rate,
		Notes:          "  note  ",
	}
	require.NoError(t, normalizePromotionChannelInput(&input))
	require.Equal(t, "FORUM-CN_01", input.Code)
	require.Equal(t, "中文论坛", input.Name)
	require.Equal(t, "论坛", input.ChannelType)
	require.Equal(t, "note", input.Notes)
}

func TestNormalizePromotionChannelInputRejectsUnsafeCodeAndRate(t *testing.T) {
	for _, input := range []PromotionChannelInput{
		{Code: "bad code", Name: "x"},
		{Code: "BAD/URL", Name: "x"},
		{Code: "OK", Name: "x", CommissionRate: float64Pointer(100.01)},
		{Code: "OK", Name: "x", CommissionRate: float64Pointer(math.NaN())},
	} {
		require.Error(t, normalizePromotionChannelInput(&input))
	}
}

func TestNormalizePromotionPromoterInput(t *testing.T) {
	valid := PromotionPromoterInput{Name: " agent ", CommissionRate: 10, CommissionFreezeDays: 7}
	require.NoError(t, normalizePromotionPromoterInput(&valid))
	require.Equal(t, "agent", valid.Name)

	for _, invalid := range []PromotionPromoterInput{
		{Name: "", CommissionRate: 10, CommissionFreezeDays: 7},
		{Name: "agent", CommissionRate: -1, CommissionFreezeDays: 7},
		{Name: "agent", CommissionRate: 10, CommissionFreezeDays: 366},
	} {
		require.Error(t, normalizePromotionPromoterInput(&invalid))
	}
}

func float64Pointer(value float64) *float64 { return &value }
