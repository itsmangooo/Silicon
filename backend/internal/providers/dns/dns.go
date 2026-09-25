package dns

import "context"

type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type Record struct {
	ID      string `json:"id"`
	ZoneID  string `json:"zoneId"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}
type DesiredRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

type Provider interface {
	TestConnection(context.Context) error
	ListZones(context.Context, string) ([]Zone, error)
	FindRecords(context.Context, string, string) ([]Record, error)
	CreateRecord(context.Context, string, DesiredRecord) (Record, error)
	UpdateRecord(context.Context, string, string, DesiredRecord) (Record, error)
	DeleteRecord(context.Context, string, string) error
}
