package phonelab

// Tag: Activity-LifeCycle-QoE
// Example:
// {
//	"Action":"onStart",
//	"AppName":"com.google.android.googlequicksearchbox",
//	"Pid":1836,"Uid":10035,
//	"Tid":1836,
//	"ParentActivity":"NULL",
//	"ActivityName":"com.google.android.googlequicksearchbox\/com.google.android.launcher.GEL",
//	"Time":1488389074792,
//	"UpTime":30472,
//	"SessionID":"f8593374-df52-4a4f-a04c-6690d68d4026",
//	"timestamp":1488389074792,
//	"uptimeNanos":30472084570,
//	"LogFormat":"1.1"
// }
type QoEActivityLifecycleLog struct {
	PLLog
	Action         string `json:"Action"`
	AppName        string `json:"AppName"`
	Pid            int    `json:"Pid"`
	Uid            int    `json:"Uid"`
	Tid            int    `json:"Tid"`
	ActivityName   string `json:"ActivityName"`
	ParentActivity string `json:"ParentActivity"`
	TimeMs         uint64 `json:"Time"`
	UpTimeMs       uint64 `json:"UpTime"`
	SessionID      string `json:"SessionID"`
}

type QoEActivityLifecylcleProps struct {
}

func (p *QoEActivityLifecylcleProps) New() interface{} {
	return &QoEActivityLifecycleLog{}
}

func NewQoEActivityLifecycleParser() Parser {
	return NewJSONParser(&QoEActivityLifecylcleProps{})
}
