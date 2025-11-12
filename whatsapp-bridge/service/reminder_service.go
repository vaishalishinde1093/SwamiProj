package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	"whatsapp-client/config"
	"whatsapp-client/domain"
	"whatsapp-client/repository"
)

// ReminderService handles sending seva completion reminders
type ReminderService struct {
	csvRepo      *repository.CSVRepository
	whatsappSvc  WhatsAppClient
	config       *config.Config
	messageStore MessageStore
}

// NewReminderService creates a new reminder service
func NewReminderService(
	csvRepo *repository.CSVRepository,
	whatsappSvc WhatsAppClient,
	cfg *config.Config,
	msgStore MessageStore,
) *ReminderService {
	return &ReminderService{
		csvRepo:      csvRepo,
		whatsappSvc:  whatsappSvc,
		config:       cfg,
		messageStore: msgStore,
	}
}

// ReminderResult contains the result of sending reminders
type ReminderResult struct {
	TotalMembers       int
	RemindersAttempted int
	RemindersSent      int
	RemindersFailed    int
	Details            []MemberReminderStatus
}

// MemberReminderStatus tracks reminder status for a member
type MemberReminderStatus struct {
	Name        string
	PhoneNumber string
	AdhyayNo    int
	Status      string // "sent", "failed", "skipped"
	Error       string
}

// getGroupConfig retrieves configuration for a specific group
func (rs *ReminderService) getGroupConfig(sevaType domain.SevaType, groupNo int) (*domain.SevaGroup, error) {
	groups, ok := rs.config.Groups[string(sevaType)]
	if !ok {
		return nil, fmt.Errorf("seva type %s not found in configuration", sevaType)
	}
	for _, cfg := range groups {
		if cfg.Number == groupNo {
			return &domain.SevaGroup{
				Number:      cfg.Number,
				JID:         cfg.JID,
				Name:        cfg.Name,
				Type:        sevaType,
				CSVPath:     cfg.CSVPath,
				MaxAdhyas:   cfg.MaxAdhyas,
				MaxPollSize: cfg.MaxPollSize,
			}, nil
		}
	}
	return nil, fmt.Errorf("group %d not found for seva type %s", groupNo, sevaType)
}

// SendRemindersAutomatic automatically detects who hasn't voted by analyzing recent group messages
// NEW APPROACH: Match CSV member names with actual poll option names that were voted for
func (rs *ReminderService) SendRemindersAutomatic(sevaType domain.SevaType, groupNo int, customMessage string) (*ReminderResult, error) {
	log.Printf("⚡ Automatically detecting members who haven't completed seva for %s group %d", sevaType, groupNo)
	// Get group configuration
	groupConfig, err := rs.getGroupConfig(sevaType, groupNo)
	if err != nil {
		return nil, fmt.Errorf("failed to get group config: %w", err)
	}

	// Read all members from CSV
	allMembers, err := rs.csvRepo.ReadMembers(groupConfig.CSVPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read members: %w", err)
	}

	// Get completed member names from TODAY's poll votes (option names that were voted for)
	completedMemberNames, err := rs.messageStore.GetCompletedMembersNames(groupConfig.JID)
	if err != nil {
		log.Printf("⚠️ Failed to get completed members: %v. Fallback to remind everyone", err)
		return rs.sendRemindersToAllMembers(allMembers, sevaType, groupNo, customMessage)
	}

	log.Printf("⚡ Found %d completed member names from TODAY's poll votes", len(completedMemberNames))

	// Find members whose names are NOT in the completed set
	nonCompletedMembers := rs.findMembersNotCompleted(allMembers, completedMemberNames)
	log.Printf("⚡ Identified %d members who haven't completed seva", len(nonCompletedMembers))

	if len(nonCompletedMembers) == 0 {
		log.Printf("✅ All members appear to have completed their seva!")
		return &ReminderResult{
			TotalMembers:       len(allMembers),
			RemindersAttempted: 0,
			RemindersSent:      0,
			RemindersFailed:    0,
			Details:            []MemberReminderStatus{},
		}, nil
	}

	// Prepare reminder message
	message := rs.buildReminderMessage(sevaType, groupNo, customMessage)

	// Send reminders only to non-completed members
	return rs.sendRemindersToMembers(nonCompletedMembers, message, sevaType, groupNo, len(allMembers))
}

// findMembersNotCompleted identifies CSV members whose names are NOT in the completed set
// Uses fuzzy name matching to handle slight variations in names
func (rs *ReminderService) findMembersNotCompleted(allMembers []domain.Member, completedNames map[string]bool) []domain.Member {
	nonCompleted := make([]domain.Member, 0)
	for _, member := range allMembers {
		if member.PhoneNumber == "" {
			log.Printf("⚡ Skipping %s - no phone number", member.Name)
			continue
		}
		// Check if this member's name appears in completed names
		// Try exact match first (with trimming)
		memberName := strings.TrimSpace(member.Name)
		isCompleted := completedNames[memberName]

		// Also try case-insensitive match
		if !isCompleted {
			memberNameLower := strings.ToLower(memberName)
			for completedName := range completedNames {
				if strings.ToLower(strings.TrimSpace(completedName)) == memberNameLower {
					isCompleted = true
					break
				}
			}
		}
		if !isCompleted {
			nonCompleted = append(nonCompleted, member)
			log.Printf("⚡ Member hasn't completed seva: %s", member.Name)
		} else {
			log.Printf("✅ %s has completed seva (name found in poll votes)", member.Name)
		}
	}
	return nonCompleted
}

// SendRemindersToAll sends reminders to ALL members with phone numbers
// Use this at 4pm to remind everyone (broadcast mode)
func (rs *ReminderService) SendRemindersToAll(sevaType domain.SevaType, groupNo int, customMessage string) (*ReminderResult, error) {
	log.Printf("⚡ Sending reminders to ALL members of %s group %d", sevaType, groupNo)
	// Get group configuration
	groupConfig, err := rs.getGroupConfig(sevaType, groupNo)
	if err != nil {
		return nil, fmt.Errorf("failed to get group config: %w", err)
	}

	// Read all members from CSV
	allMembers, err := rs.csvRepo.ReadMembers(groupConfig.CSVPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read members: %w", err)
	}

	return rs.sendRemindersToAllMembers(allMembers, sevaType, groupNo, customMessage)
}

// sendRemindersToAllMembers is a helper to send reminders to all members
func (rs *ReminderService) sendRemindersToAllMembers(allMembers []domain.Member, sevaType domain.SevaType, groupNo int, customMessage string) (*ReminderResult, error) {
	// Filter only members with phone numbers
	membersToRemind := make([]domain.Member, 0)
	for _, m := range allMembers {
		if m.PhoneNumber != "" {
			membersToRemind = append(membersToRemind, m)
		} else {
			log.Printf("⚡ Skipping %s - no phone number", m.Name)
		}
	}

	// Prepare reminder message
	message := rs.buildReminderMessage(sevaType, groupNo, customMessage)

	// Send reminders
	return rs.sendRemindersToMembers(membersToRemind, message, sevaType, groupNo, len(allMembers))
}

// SendRemindersToSpecific sends reminders to specific members by name
// You can check WhatsApp to see who hasn't voted and pass their names
func (rs *ReminderService) SendRemindersToSpecific(sevaType domain.SevaType, groupNo int, memberNames []string, customMessage string) (*ReminderResult, error) {
	log.Printf("⚡ Sending reminders to %d specific members of %s group %d", len(memberNames), sevaType, groupNo)
	if len(memberNames) == 0 {
		return nil, fmt.Errorf("no member names provided")
	}

	// Get group configuration
	groupConfig, err := rs.getGroupConfig(sevaType, groupNo)
	if err != nil {
		return nil, fmt.Errorf("failed to get group config: %w", err)
	}

	// Read all members from CSV
	allMembers, err := rs.csvRepo.ReadMembers(groupConfig.CSVPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read members: %w", err)
	}

	// Create set of names to remind (case-insensitive)
	nameSet := make(map[string]bool)
	for _, name := range memberNames {
		nameSet[strings.ToLower(strings.TrimSpace(name))] = true
	}

	// Filter members by name
	membersToRemind := make([]domain.Member, 0)
	for _, m := range allMembers {
		if nameSet[strings.ToLower(m.Name)] {
			if m.PhoneNumber != "" {
				membersToRemind = append(membersToRemind, m)
			} else {
				log.Printf("⚡ Cannot remind %s - no phone number", m.Name)
			}
		}
	}
	if len(membersToRemind) == 0 {
		return nil, fmt.Errorf("no members found with provided names and phone numbers")
	}

	// Prepare reminder message
	message := rs.buildReminderMessage(sevaType, groupNo, customMessage)

	return rs.sendRemindersToMembers(membersToRemind, message, sevaType, groupNo, len(allMembers))
}

// sendRemindersToMembers sends reminders to a list of members
func (rs *ReminderService) sendRemindersToMembers(members []domain.Member, baseMessage string, sevaType domain.SevaType, groupNo, totalMembers int) (*ReminderResult, error) {
	result := &ReminderResult{
		TotalMembers:       totalMembers,
		RemindersAttempted: len(members),
		Details:            make([]MemberReminderStatus, 0, len(members)),
	}
	for _, member := range members {
		status := rs.sendReminderToMember(member, baseMessage)
		result.Details = append(result.Details, status)
		switch status.Status {
		case "sent":
			result.RemindersSent++
		case "failed":
			result.RemindersFailed++
		}
	}
	log.Printf("⚡ Reminders complete: %d sent, %d failed out of %d attempted", result.RemindersSent, result.RemindersFailed, result.RemindersAttempted)
	return result, nil
}

// buildReminderMessage creates the reminder message
func (rs *ReminderService) buildReminderMessage(sevaType domain.SevaType, groupNo int, customMessage string) string {
	if customMessage != "" {
		return customMessage
	}
	// Get current day in Marathi and seva name
	//marathiDay := getMarathiDayOfWeek()
	sevaName := rs.getSevaDisplayName(sevaType)

	// Combine day and seva name for the placeholder
	//dayAndSeva := fmt.Sprintf("%s %s", marathiDay, sevaName)

	// New reminder message template
	template := `आजची आपली %s सेवा अजून बाकी आहे. तरी कृपया लवकर पूर्ण करावी. संध्याकाळी सहा वाजेच्या आत सेवा पूर्ण करून पोलला (Poll) टिक करावे. तुमची सेवा पूर्ण झाल्याशिवाय ग्रुप पूर्ण होऊ शकत नाही, म्हणून सहकार्य करावे.

आपली सेवा पूर्ण झाली असल्यास, कृपया खालील पोलवर क्लिक करा:

श्री स्वामी समर्थ 🙏
मिनाक्षी बागुल 🙏`

	return fmt.Sprintf(template, sevaName)
}

// sendReminderToMember sends a reminder to a single member
func (rs *ReminderService) sendReminderToMember(member domain.Member, baseMessage string) MemberReminderStatus {
	status := MemberReminderStatus{
		Name:        member.Name,
		PhoneNumber: member.PhoneNumber,
		AdhyayNo:    member.AdhyayNo,
	}

	// Check if member has phone number
	if member.PhoneNumber == "" {
		status.Status = "skipped"
		status.Error = "no phone number"
		log.Printf("⚡ Skipped %s - no phone number", member.Name)
		return status
	}

	// Personalize message with member's adhya number
	personalizedMessage := fmt.Sprintf("%s\nअध्याय क्रमांक %d", baseMessage, member.AdhyayNo)

	// Clean phone number (remove + prefix if present)
	phoneNumber := strings.TrimPrefix(member.PhoneNumber, "+")

	// Send WhatsApp message (no media)
	success, message := rs.whatsappSvc.SendMessage(phoneNumber, personalizedMessage, "")
	if !success {
		status.Status = "failed"
		status.Error = message
		log.Printf("⚡ Failed to send reminder to %s (%s): %s", member.Name, member.PhoneNumber, status.Error)
		return status
	}
	status.Status = "sent"
	log.Printf("✅ Reminder sent to %s (%s)", member.Name, member.PhoneNumber)
	return status
}

func getMarathiDayOfWeek() string {
	days := map[time.Weekday]string{
		time.Sunday:    "रविवार",
		time.Monday:    "सोमवार",
		time.Tuesday:   "मंगळवार",
		time.Wednesday: "बुधवार",
		time.Thursday:  "गुरुवार",
		time.Friday:    "शुक्रवार",
		time.Saturday:  "शनिवार",
	}
	return days[time.Now().Weekday()]
}

func getMarathiDayWithDate() string {
	// Example: "गुरुवार, 27/10/2025"
	tomorrow := time.Now().AddDate(0, 0, 1) // Get tomorrow's date
	days := map[time.Weekday]string{
		time.Sunday:    "रविवार",
		time.Monday:    "सोमवार",
		time.Tuesday:   "मंगळवार",
		time.Wednesday: "बुधवार",
		time.Thursday:  "गुरुवार",
		time.Friday:    "शुक्रवार",
		time.Saturday:  "शनिवार",
	}
	dayName := days[tomorrow.Weekday()]
	dateStr := tomorrow.Format("02/01/2006") // DD/MM/YYYY format
	return fmt.Sprintf("%s, %s", dayName, dateStr)
}

func (rs *ReminderService) getSevaDisplayName(sevaType domain.SevaType) string {
	switch sevaType {
	case domain.SevaTypeEkadashiBhagavat:
		return "एकादशी भागवत"
	case domain.SevaTypeDurgaPaath:
		return "दुर्गा पाठ"
	case domain.SevaTypeSaptahikSwami:
		return "साप्ताहिक स्वामी"
	case domain.SevaTypeDarbar:
		return "साप्ताहिक दुर्गा"
	case domain.SevaTypeMalhari:
		return "मल्हारी महात्म"
	default:
		return string(sevaType)
	}
}

func (rs *ReminderService) SendGroupAnnouncement(sevaType domain.SevaType, groupNo int) (string, error) {
	log.Printf("⚡ Sending group announcement for %s group %d", sevaType, groupNo)

	// Get group configuration
	groupConfig, err := rs.getGroupConfig(sevaType, groupNo)
	if err != nil {
		return "", fmt.Errorf("failed to get group config: %w", err)
	}

	// Read all members from CSV
	allMembers, err := rs.csvRepo.ReadMembers(groupConfig.CSVPath)
	if err != nil {
		return "", fmt.Errorf("failed to read members: %w", err)
	}

	// Get completed member names from TODAY's poll votes
	completedMemberNames, err := rs.messageStore.GetCompletedMembersNames(groupConfig.JID)
	if err != nil {
		log.Printf("⚠️ Failed to get completed members: %v. Assuming all need reminders.", err)
		completedMemberNames = make(map[string]bool)
	}

	log.Printf("⚡ Found %d completed member names from TODAY's poll votes", len(completedMemberNames))

	// Find members whose names are NOT in the completed set
	nonCompletedMembers := rs.findMembersNotCompleted(allMembers, completedMemberNames)
	log.Printf("⚡ Identified %d members who haven't completed seva", len(nonCompletedMembers))
	if len(nonCompletedMembers) == 0 {
		return "", nil
	}

	// Build list of member names
	var namesList []string
	for _, member := range nonCompletedMembers {
		namesList = append(namesList, member.Name)
	}
	namesStr := strings.Join(namesList, ", ")

	// Create message with names separated by commas
	message := fmt.Sprintf("खालील सेवेकर्यांचीं सेवा बाकी आहे, सदस्य: %s\n\nकृपया लवकरात लवकर सेवा करावी आणि WhatsApp ग्रुपवर poll ला प्रतिसाद द्यावा \nश्री स्वामी समर्थ 🙏 \nमिनाक्षी बागुल 🙏", namesStr)

	// Send message to group
	success, statusMsg := rs.whatsappSvc.SendMessage(groupConfig.JID, message, "")
	if !success {
		return "", fmt.Errorf("failed to send message: %s", statusMsg)
	}

	log.Printf("✅ Group announcement sent: %d members pending", len(nonCompletedMembers))
	return message, nil
}
