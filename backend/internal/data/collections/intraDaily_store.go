package collections

type IntraDailyStore interface {
	Init() error
	CreateIntraDaily(intraDaily *IntraDailyRequest, response *IntraDailyResponse) (*IntraDailyResponse, error)
	GetIntraDailyByRequest(intraDaily *IntraDailyRequest) (*IntraDailyResponse, error)
}
