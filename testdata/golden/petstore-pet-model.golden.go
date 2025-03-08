type Pet struct {
	Category Category `json:"category,omitempty"`
	Id int64 `json:"id"`
	Name string `json:"name"`
	// pet status in the store
	Status string `json:"status,omitempty"`
	Tag string `json:"tag,omitempty"`
}
