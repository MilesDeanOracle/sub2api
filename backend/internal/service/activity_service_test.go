package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestActivityDayUsesShanghaiCalendar(t *testing.T) {
	day, month := activityDay(time.Date(2026, 12, 31, 16, 30, 0, 0, time.UTC))
	if day != "2027-01-01" || month != "2027-01" {
		t.Fatalf("got %s %s", day, month)
	}
}

func TestCheckinRewardCaps(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		streak, daily, milestone, cap, spent int
		budget, global                       int64
		want                                 int
	}{
		{"daily", 1, 10, 50, 1500, 0, 10000, 0, 10},
		{"milestone", 7, 10, 50, 1500, 60, 10000, 0, 60},
		{"monthly cap", 7, 10, 50, 1500, 1495, 10000, 0, 5},
		{"global budget", 7, 10, 50, 1500, 0, 100, 97, 3},
		{"exhausted", 7, 10, 50, 1500, 1500, 100, 100, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculateCheckinReward(tc.streak, tc.daily, tc.milestone, tc.cap, tc.spent, tc.budget, tc.global); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestActivityCheckinCapTracksRechargeMultiplier(t *testing.T) {
	if got := activityCheckinCap(1500, 0.5); got != 750 {
		t.Fatalf("got %d want 750", got)
	}
	if got := activityCheckinCap(1200, 2); got != 1200 {
		t.Fatalf("got %d want 1200", got)
	}
}

func TestActivityUserCap(t *testing.T) {
	if got := activityRemainingUserCap(60, 200, 0); got != 60 {
		t.Fatalf("reward got %d", got)
	}
	if got := activityRemainingUserCap(60, 100, 90); got != 10 {
		t.Fatalf("user cap got %d", got)
	}
}

func TestCheckinSpendEligibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		minimum   int64
		qualifies bool
	}{
		{"zero needs no calls", 0, true}, {"below threshold", 100, false}, {"threshold reached", 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if tc.minimum > 0 {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE(SUM(actual_cost),0) >= $2::numeric/100 FROM usage_logs WHERE user_id=$1 AND COALESCE(billing_type,0)=0 AND actual_cost>0 AND COALESCE(request_type,0)<>4`)).WithArgs(int64(42), tc.minimum).WillReturnRows(sqlmock.NewRows([]string{"eligible"}).AddRow(tc.qualifies))
			}
			got, err := checkinSpendEligible(context.Background(), db, 42, tc.minimum)
			if err != nil || got != tc.qualifies {
				t.Fatalf("got %v, %v", got, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckinSpendEligibilityFailsOnDatabaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT COALESCE").WillReturnError(errors.New("database unavailable"))
	got, err := checkinSpendEligible(context.Background(), db, 42, 100)
	if err == nil || got {
		t.Fatalf("must not grant eligibility on database error")
	}
}
