package repository

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"whatsapp-client/domain"
)

// CSVRepository handles reading and writing member data from/to CSV files
type CSVRepository struct{}

// NewCSVRepository creates a new CSV repository
func NewCSVRepository() *CSVRepository {
	return &CSVRepository{}
}

// ReadMembers reads members from a CSV file
// CSV format: Name, Adhyay_Number, Group_Number, Phone_Number (optional)
func (*CSVRepository) ReadMembers(csvPath string) ([]domain.Member, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV %s: %w", csvPath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	var members []domain.Member
	for _, record := range records {
		// Skip header or invalid records
		if len(record) < 2 {
			continue
		}
		adhyayNo, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			continue // Skip invalid records
		}

		member := domain.Member{
			Name:     strings.TrimSpace(record[0]),
			AdhyayNo: adhyayNo,
		}

		// Read phone number if available (4th column, index 3)
		if len(record) >= 4 {
			member.PhoneNumber = strings.TrimSpace(record[3])
		}

		log.Printf("Adding member %s - %d to memberlist", member.Name, member.AdhyayNo)
		members = append(members, member)
	}

	return members, nil
}

// WriteMembers writes members to a CSV file
// CSV format: Name, Adhyay_Number, Group_Number, Phone_Number (optional)
func (*CSVRepository) WriteMembers(csvPath string, members []domain.Member, groupNo int) error {
	file, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV %s: %w", csvPath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Name", "Adhyay_Number", "Group_Number", "Phone_Number"}); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write members
	for _, member := range members {
		record := []string{
			member.Name,
			strconv.Itoa(member.AdhyayNo),
			strconv.Itoa(groupNo),
			member.PhoneNumber, // May be empty string if not provided
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	return nil
}
