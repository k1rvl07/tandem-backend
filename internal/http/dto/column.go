package dto

type CreateColumnRequest struct {
	Name string `json:"name"`
}

type UpdateColumnRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}
