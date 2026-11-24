package store

import "time"

type Plan string

const (
	PlanFree    Plan = "free"
	PlanStarter Plan = "starter"
	PlanGrowth  Plan = "growth"
	PlanScale   Plan = "scale"
)

type PlanInfo struct {
	ID     Plan   `json:"id"`
	Name   string `json:"name"`
	Price  int    `json:"price"`
	Videos int    `json:"videos"`
	Blurb  string `json:"blurb"`
}

var Plans = []PlanInfo{
	{PlanFree, "Free trial", 0, 1, "Try one ad on us"},
	{PlanStarter, "Starter", 39, 12, "For new stores testing creatives"},
	{PlanGrowth, "Growth", 99, 40, "For stores running ads weekly"},
	{PlanScale, "Scale", 249, 120, "For agencies and high-volume brands"},
}

func PlanByID(p Plan) (PlanInfo, bool) {
	for _, x := range Plans {
		if x.ID == p {
			return x, true
		}
	}
	return PlanInfo{}, false
}

type Profile struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Plan             Plan    `json:"plan"`
	StripeCustomerID *string `json:"-"`
}

type Product struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Price       string   `json:"price,omitempty"`
	Image       string   `json:"image,omitempty"`
	Images      []string `json:"images"`
}

type AdStatus string

const (
	AdPending    AdStatus = "pending"
	AdProcessing AdStatus = "processing"
	AdDone       AdStatus = "done"
	AdFailed     AdStatus = "failed"
)

func (s AdStatus) Active() bool { return s == AdPending || s == AdProcessing }

type Ad struct {
	ID       string   `json:"id"`
	Hook     string   `json:"hook"`
	Script   string   `json:"script"`
	Caption  string   `json:"caption"`
	Hashtags []string `json:"hashtags"`
	Avatar   string   `json:"avatar"`
	Status   AdStatus `json:"status"`
	VideoURL string   `json:"videoUrl,omitempty"`
	Error    string   `json:"error,omitempty"`

	VideoJobID string    `json:"-"`
	VideoKey   string    `json:"-"`
	UpdatedAt  time.Time `json:"-"`
	CreatedAt  time.Time `json:"-"`
}

type Project struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Product   Product   `json:"product"`
	CreatedAt time.Time `json:"createdAt"`
	Ads       []Ad      `json:"ads"`
	UserID    string    `json:"-"`
}

// NewAd is the content of an ad before it is persisted.
type NewAd struct {
	Hook, Script, Caption, Avatar string
	Hashtags                      []string
}
