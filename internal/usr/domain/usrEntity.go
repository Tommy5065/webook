package domain

// domain层是DDD里的领域对象，User被称为entity或者BO(Business Object )
type User struct {
	ID          int32
	Email       string
	Password    string
	Nickname    string
	Birthday    string
	Aboutme     string
	PhoneNumber string
}

type ProfileResponde struct {
	Nickname string `json:"nick_name" redis:"nickname"`
	Birthday string `json:"birthday" redis:"birthday"`
	Aboutme  string `json:"about_me" redis:"aboutme"`
	Followee int64  `json:"followee" redis:"followee"`
	Follower int64  `json:"follower" redis:"follower"`
}
