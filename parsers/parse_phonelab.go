package phonelab

// These fields are included in every PhoneLab log
type PLLog struct {
	Timestamp   uint64 `json:"timestamp"`
	UptimeNanos uint64 `json:"uptimeNanos"`
	LogFormat   string `json:"LogFormat"`
}

type BatteryProps struct {
	ChargerAcOnline       bool   `json:"chargerAcOnline"`
	ChargerUsbOnline      bool   `json:"chargerUsbOnline"`
	ChargerWirelessOnline bool   `json:"chargerWirelessOnline"`
	Status                int    `json:"Status"`
	Health                int    `json:"Health"`
	Present               bool   `json:"Present"`
	Level                 int    `json:"Level"`
	Voltage               int    `json:"Voltage"`
	Temperature           int    `json:"Temperature"`
	Technology            string `json:"Technology"`
}

type PLPowerBatteryLog struct {
	PLLog
	Action            string       `json:"Action"`
	Scale             int          `json:"Scale"`
	BatteryProperties BatteryProps `json:"BatteryProperties"`
}

type PLPowerBatteryProps struct {
}

func (p *PLPowerBatteryProps) New() interface{} {
	return &PLPowerBatteryLog{}
}

func NewPLPowerBatteryParser() Parser {
	return NewJSONParser(&PLPowerBatteryProps{})
}
