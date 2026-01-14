package repository

import "whatsapp-client/domain"

type MemberStore interface {
	GetGroupMembers(sevaType domain.SevaType, groupNo int) (members []domain.Member, version int64, err error)
	ReplaceGroupMembers(sevaType domain.SevaType, groupNo int, members []domain.Member, expectedVersion int64) (newVersion int64, err error)
	ListAllGroupMembers() ([]GroupMemberRow, error)
}

type GroupMemberRow struct {
	SevaType    domain.SevaType
	GroupNo     int
	Name        string
	AdhyayNo    int
	PhoneNumber string
}
