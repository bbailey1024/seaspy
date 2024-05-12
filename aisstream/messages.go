package aisstream

// PositionReport - Class A AIS Position Report (Messages 1, 2, and 3)
// Reference: https://www.navcen.uscg.gov/ais-class-a-reports
type PositionReport struct {
	Cog                       float64 `json:"Cog"`
	CommunicationState        int     `json:"CommunicationState"`
	Latitude                  float64 `json:"Latitude"`
	Longitude                 float64 `json:"Longitude"`
	MessageID                 int     `json:"MessageID"`
	NavigationalStatus        int     `json:"NavigationalStatus"`
	PositionAccuracy          bool    `json:"PositionAccuracy"`
	Raim                      bool    `json:"Raim"`
	RateOfTurn                int     `json:"RateOfTurn"`
	RepeatIndicator           int     `json:"RepeatIndicator"`
	Sog                       float64 `json:"Sog"`
	Spare                     int     `json:"Spare"`
	SpecialManoeuvreIndicator int     `json:"SpecialManoeuvreIndicator"`
	Timestamp                 int     `json:"Timestamp"`
	TrueHeading               int     `json:"TrueHeading"`
	UserID                    int     `json:"UserID"`
	Valid                     bool    `json:"Valid"`
}

// ShipStaticData - Class A Ship Static and Voyage Related Data (Message 5)
// Reference: https://www.navcen.uscg.gov/ais-class-a-static-voyage-message-5
type ShipStaticData struct {
	AisVersion  int    `json:"AisVersion"`
	CallSign    string `json:"CallSign"`
	Destination string `json:"Destination"`
	Dimension   struct {
		A int `json:"A"`
		B int `json:"B"`
		C int `json:"C"`
		D int `json:"D"`
	} `json:"Dimension"`
	Dte bool `json:"Dte"`
	Eta struct {
		Day    int `json:"Day"`
		Hour   int `json:"Hour"`
		Minute int `json:"Minute"`
		Month  int `json:"Month"`
	} `json:"Eta"`
	FixType              int     `json:"FixType"`
	ImoNumber            int     `json:"ImoNumber"`
	MaximumStaticDraught float64 `json:"MaximumStaticDraught"`
	MessageID            int     `json:"MessageID"`
	Name                 string  `json:"Name"`
	RepeatIndicator      int     `json:"RepeatIndicator"`
	Spare                bool    `json:"Spare"`
	Type                 int     `json:"Type"`
	UserID               int     `json:"UserID"`
	Valid                bool    `json:"Valid"`
}
