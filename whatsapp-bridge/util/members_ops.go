package util

import (
	"sort"
	"whatsapp-client/domain"
)

// SortByAdhyay sorts members by their adhyay number
func SortByAdhyay(members []domain.Member) []domain.Member {
	sorted := make([]domain.Member, len(members))
	copy(sorted, members)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AdhyayNo < sorted[j].AdhyayNo
	})

	return sorted
}

// RotateAdhyas rotates adhyay numbers by 1 (with wrap-around)
func RotateAdhyas(members []domain.Member, maxAdhyas int) []domain.Member {
	rotated := make([]domain.Member, len(members))

	for i, member := range members {
		newAdhyay := member.AdhyayNo + 1
		if newAdhyay > maxAdhyas {
			newAdhyay = 1
		}
		rotated[i] = domain.Member{
			Name:        member.Name,
			AdhyayNo:    newAdhyay,
			PhoneNumber: member.PhoneNumber, // ✅ Preserve phone number!
		}
	}

	return rotated
}
