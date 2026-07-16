package domain

type FeedArticleItem struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
	ReadUrl    string `json:"read_url"`
	LikesCount int64  `json:"likes_count"`
	IsLiked    bool   `json:"is_liked"`
}

type ListLikesCursor struct {
	LikesCount int64
	IdBefore   int64
}

type ListLikesResponde struct {
	ArticleList      []FeedArticleItem
	LikeCountsBefore *int64 `json:"next_likes_count_before,omitempty"`
	IdBefore         *int64 `json:"next_id_before,omitempty"`
	HasMore          bool   `json:"has_more"`
}

type ListFollowCursor struct {
	TimeBefore int64
	IdBefore   int64
}

type ListFollowResponde struct {
	ArticleList    []FeedArticleItem
	LastTimeBefore *int64 `json:"next_follow_before,omitempty"`
	IdBefore       *int64 `json:"next_id_before,omitempty"`
}
