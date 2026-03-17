package domain

// Member represents a seva participant (unified model for all seva types)
type Member struct {
	Name        string `json:"name"`
	AdhyayNo    int    `json:"adhyay_no"`
	PhoneNumber string `json:"phone_number,omitempty"` // Optional - for sending reminders
}

// SevaType represents the type of seva
type SevaType string

const (
	SevaTypeEkadashiBhagavat SevaType = "ekadashi_bhagavat"
	SevaTypeDurgaPaath       SevaType = "durga_paath"
	SevaTypeSaptahikSwami    SevaType = "saptahik_swami"
	SevaTypeMalhari          SevaType = "malhari"
	SevaTypeDarbar           SevaType = "darbar"
	SevaTypeChaitraNavratri  SevaType = "chaitra_navratri"
)

// SevaGroup represents a seva group configuration
type SevaGroup struct {
	Number      int
	JID         string
	Name        string
	Type        SevaType
	CSVPath     string
	MaxAdhyas   int
	MaxPollSize int
}

// GroupAssignment represents member assignments for a specific group
type GroupAssignment struct {
	Group   SevaGroup
	Members []Member
	Date    string
}
