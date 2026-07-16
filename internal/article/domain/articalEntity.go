package domain

type Artical struct {
	Title   string
	Content string
	Status  ArticalStatus
	ArtiID  int64
	Author  Author
	Utime   int64
	Ctime   int64
	New     bool
}

type ArticalStatus int

const (
	ArticalPublished ArticalStatus = 1  // 表示正常发表
	ArticalHidend    ArticalStatus = 2  // 表示帖子仅自己可见
	ArticalDeleted   ArticalStatus = 3  // 表示帖子已删除
	ArticalUnKnow    ArticalStatus = -1 // 表示帖子处于未知状态
	ArticalUnPublish ArticalStatus = 0  // 未发表
)

type Author struct {
	ID   int32  `json:"author_id,omitempty"`
	Name string `json:"author_name,omitempty"`
}

type ArticleSearchRespond struct {
	ID         int64  `json:"article_id"`
	Title      string `json:"title"`
	Content    string `json:"content,omitempty"`
	AuthorInfo Author
	LikeCount  int64  `json:"like_count"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
}
