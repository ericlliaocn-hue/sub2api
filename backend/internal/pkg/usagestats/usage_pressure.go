package usagestats

import "time"

const (
	PressureLevelIdle     = "idle"
	PressureLevelWarm     = "warm"
	PressureLevelBusy     = "busy"
	PressureLevelScramble = "scramble"
)

// UsagePressureWindow is billed usage inside a live lookback window.
type UsagePressureWindow struct {
	Requests int64   `json:"requests"`
	Users    int64   `json:"users"`
	Accounts int64   `json:"accounts"`
	Billed   float64 `json:"billed"`
}

// UsagePressureWindows groups the 5 / 15 / 60 minute live windows.
type UsagePressureWindows struct {
	M5  UsagePressureWindow `json:"m5"`
	M15 UsagePressureWindow `json:"m15"`
	M60 UsagePressureWindow `json:"m60"`
}

// UsagePressureActor is a user or account seen in the live window.
type UsagePressureActor struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Requests int64     `json:"requests"`
	Billed   float64   `json:"billed"`
	LastAt   time.Time `json:"last_at"`
}

// UsagePressurePeakHour is today's highest-billed clock hour so far.
type UsagePressurePeakHour struct {
	Hour     time.Time `json:"hour"`
	Requests int64     `json:"requests"`
	Users    int64     `json:"users"`
	Billed   float64   `json:"billed"`
}

// UsagePressureSnapshot is the live "how's the pressure" board payload.
type UsagePressureSnapshot struct {
	GeneratedAt   time.Time              `json:"generated_at"`
	Level         string                 `json:"level"`
	HourlyFrom15m float64                `json:"hourly_from_15m"`
	PeakRatio     float64                `json:"peak_ratio"`
	Windows       UsagePressureWindows   `json:"windows"`
	Today         UsagePressureWindow    `json:"today"`
	PeakHour      *UsagePressurePeakHour `json:"peak_hour,omitempty"`
	Users         []UsagePressureActor   `json:"users"`
	Accounts      []UsagePressureActor   `json:"accounts"`
}

// ClassifyUsagePressure maps 15-minute burn against today's peak hour.
// When today has no meaningful peak yet, it falls back to absolute 15-minute traffic.
func ClassifyUsagePressure(users15 int64, billed15, peakHourBilled float64) string {
	if users15 <= 0 && billed15 <= 0 {
		return PressureLevelIdle
	}
	hourly := billed15 * 4
	if peakHourBilled > 0.01 {
		ratio := hourly / peakHourBilled
		switch {
		case ratio >= 0.85:
			return PressureLevelScramble
		case ratio >= 0.45:
			return PressureLevelBusy
		case ratio >= 0.15:
			return PressureLevelWarm
		default:
			return PressureLevelIdle
		}
	}
	switch {
	case users15 >= 12 || hourly >= 40:
		return PressureLevelScramble
	case users15 >= 6 || hourly >= 15:
		return PressureLevelBusy
	case users15 >= 2 || hourly >= 4:
		return PressureLevelWarm
	default:
		return PressureLevelIdle
	}
}

// FinalizeUsagePressure fills derived mood / rate fields from window + peak data.
func FinalizeUsagePressure(snap *UsagePressureSnapshot) {
	if snap == nil {
		return
	}
	if snap.Users == nil {
		snap.Users = []UsagePressureActor{}
	}
	if snap.Accounts == nil {
		snap.Accounts = []UsagePressureActor{}
	}
	snap.HourlyFrom15m = snap.Windows.M15.Billed * 4
	peak := 0.0
	if snap.PeakHour != nil {
		peak = snap.PeakHour.Billed
	}
	if peak > 0.01 {
		snap.PeakRatio = snap.HourlyFrom15m / peak
	} else {
		snap.PeakRatio = 0
	}
	snap.Level = ClassifyUsagePressure(snap.Windows.M15.Users, snap.Windows.M15.Billed, peak)
}

// RedactUsagePressureForUser drops who/which-account details so the public board
// only shows pool heat, not other users or account names.
func RedactUsagePressureForUser(snap *UsagePressureSnapshot) *UsagePressureSnapshot {
	if snap == nil {
		return &UsagePressureSnapshot{
			Users:    []UsagePressureActor{},
			Accounts: []UsagePressureActor{},
		}
	}
	out := *snap
	out.Users = []UsagePressureActor{}
	out.Accounts = []UsagePressureActor{}
	return &out
}
