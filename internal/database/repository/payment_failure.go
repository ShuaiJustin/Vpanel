package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaymentFailureCreate   = "gateway_create"
	PaymentFailureRejected = "gateway_rejected"
	PaymentFailureQuery    = "gateway_query"
	PaymentFailureMismatch = "details_mismatch"
	PaymentFailureApply    = "local_apply"
)

// PaymentFailure stores the latest occurrence of one reason per order and channel.
type PaymentFailure struct {
	OrderNo    string    `gorm:"primaryKey;size:64" json:"order_no"`
	Method     string    `gorm:"primaryKey;size:32" json:"method"`
	Reason     string    `gorm:"primaryKey;size:32" json:"reason"`
	LastSeenAt time.Time `gorm:"index" json:"last_seen_at"`
}

type PaymentFailureReport struct {
	WindowDays int                       `json:"window_days" gorm:"-"`
	Orders     int64                     `json:"orders"`
	Unresolved int64                     `json:"unresolved"`
	Recent     []PaymentFailureReportRow `json:"recent" gorm:"-"`
}

type PaymentFailureReportRow struct {
	PaymentFailure
	Resolved bool `json:"resolved"`
}

type PaymentFailureRepository struct {
	db *gorm.DB
}

func NewPaymentFailureRepository(db *gorm.DB) *PaymentFailureRepository {
	return &PaymentFailureRepository{db: db}
}

func (r *PaymentFailureRepository) Record(ctx context.Context, orderNo, method, reason string, now time.Time) error {
	entry := PaymentFailure{OrderNo: orderNo, Method: method, Reason: reason, LastSeenAt: now}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "order_no"}, {Name: "method"}, {Name: "reason"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_seen_at": gorm.Expr("CASE WHEN payment_failures.last_seen_at < ? THEN ? ELSE payment_failures.last_seen_at END", now, now),
		}),
	}).Create(&entry).Error
}

func (r *PaymentFailureRepository) Report(ctx context.Context, now time.Time) (PaymentFailureReport, error) {
	const windowDays = 30
	report := PaymentFailureReport{Recent: []PaymentFailureReportRow{}}
	// Preserve verified discrepancies for an operator even if another channel paid.
	resolved := `COALESCE((CASE WHEN f.reason IN ('gateway_create', 'gateway_query', 'gateway_rejected')
		THEN (o.paid_at IS NOT NULL OR b.paid_at IS NOT NULL)
		WHEN f.reason = 'local_apply'
		THEN ((o.payment_method = f.method AND o.paid_at IS NOT NULL AND o.fulfillment_pending = false)
		  OR (b.method = f.method AND b.paid_at IS NOT NULL))
		ELSE false END), false)`
	query := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("payment_failures AS f").
			Joins("LEFT JOIN orders AS o ON o.order_no = f.order_no").
			Joins("LEFT JOIN balance_recharge_orders AS b ON b.order_no = f.order_no").
			Where("f.last_seen_at >= ?", now.AddDate(0, 0, -windowDays))
	}
	if err := query().Select("COUNT(DISTINCT f.order_no) AS orders, " +
		"COUNT(DISTINCT CASE WHEN " + resolved + " = false THEN f.order_no END) AS unresolved").Scan(&report).Error; err != nil {
		return report, err
	}
	report.WindowDays = windowDays
	err := query().Select("f.order_no, f.method, f.reason, f.last_seen_at, " + resolved + " AS resolved").
		Order("f.last_seen_at DESC, f.order_no, f.method, f.reason").Limit(20).Scan(&report.Recent).Error
	return report, err
}
