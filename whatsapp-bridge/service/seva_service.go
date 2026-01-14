package service

import (
	"fmt"
	"log"
	"sort"
	"time"

	"whatsapp-client/config"
	"whatsapp-client/domain"
	"whatsapp-client/repository"
)

// SevaService handles seva automation (sending messages and polls)
type SevaService struct {
	csvRepo     *repository.CSVRepository
	whatsAppSvc WhatsAppClient
	config      *config.Config
}

// NewSevaService creates a new seva service
func NewSevaService(
	csvRepo *repository.CSVRepository,
	whatsAppSvc WhatsAppClient,
	config *config.Config,
) *SevaService {
	return &SevaService{
		csvRepo:     csvRepo,
		whatsAppSvc: whatsAppSvc,
		config:      config,
	}
}

// SendSevaAutomation sends both a message and poll for seva automation
func (ss *SevaService) SendSevaAutomation(sevaType domain.SevaType, groupNo int) error {
	log.Printf("🚦 Starting seva automation for %s group %d", sevaType, groupNo)

	// Get group configuration
	groupConfig, err := ss.getGroupConfig(sevaType, groupNo)
	if err != nil {
		return fmt.Errorf("failed to get group config: %w", err)
	}

	// Read members from CSV
	members, err := ss.csvRepo.ReadMembers(groupConfig.CSVPath)

	if err != nil {
		return fmt.Errorf("failed to read members: %w", err)
	}
	if len(members) == 0 {
		return fmt.Errorf("no members found in CSV for group %d", groupNo)
	}
	log.Printf("🚦 Found %d members in CSV.", len(members))

	// Build message
	message := ss.buildSevaMessage(sevaType, groupNo, members)
	// Send message to group
	log.Printf("Message to be sent : %s", message)
	success, statusMsg := ss.whatsAppSvc.SendMessage(groupConfig.JID, message, "")
	if !success {
		return fmt.Errorf("failed to send message: %s", statusMsg)
	}
	log.Printf("✅ Message sent successfully with status : %s", statusMsg)

	// Build poll options from members (with rotated adhyay numbers)
	pollOptions := ss.buildPollOptions(members, sevaType)

	// Split into multiple polls if needed (WhatsApp max is 12 options per poll)
	maxOptionsPerPoll := groupConfig.MaxPollSize
	totalMembers := len(pollOptions)
	if totalMembers <= maxOptionsPerPoll {
		pollName := fmt.Sprintf("%s - ग्रुप %d", getSevaDisplayName(sevaType), groupNo)
		selectableCount := uint32(len(pollOptions))
		log.Printf("🗳️ Sending poll with %d options (selectable: %d)...", len(pollOptions), selectableCount)
		success, statusMsg := ss.whatsAppSvc.SendPoll(groupConfig.JID, pollName, pollOptions, selectableCount)
		if !success {
			return fmt.Errorf("failed to send poll: %s", statusMsg)
		}
		log.Printf("✅ Poll sent successfully : %s", statusMsg)
	} else {
		// Multiple polls needed - distribute members equally across 2 polls
		numPolls := 2
		firstPollSize := (totalMembers + 1) / 2 // ceiling division
		secondPollSize := totalMembers / 2      // floor division
		log.Printf("🗳️ Splitting %d members into %d polls (first: %d, second: %d)...", totalMembers, numPolls, firstPollSize, secondPollSize)

		// Send first poll
		pollOptionsBatch1 := pollOptions[:firstPollSize]
		pollName := fmt.Sprintf("%s - ग्रुप %d - Poll 1", getSevaDisplayName(sevaType), groupNo)
		selectableCount := uint32(len(pollOptionsBatch1))
		log.Printf("🗳️ Sending poll 1/%d with %d options (selectable: %d)...", numPolls, len(pollOptionsBatch1), selectableCount)
		success, statusMsg := ss.whatsAppSvc.SendPoll(groupConfig.JID, pollName, pollOptionsBatch1, selectableCount)
		if !success {
			return fmt.Errorf("failed to send poll 1/%d: %s", numPolls, statusMsg)
		}
		log.Printf("✅ Poll 1/%d sent successfully", numPolls)

		// Small delay between polls
		time.Sleep(1 * time.Second)

		// Send second poll
		pollOptionsBatch2 := pollOptions[firstPollSize:]
		pollName = fmt.Sprintf("%s - ग्रुप %d - Poll 2", getSevaDisplayName(sevaType), groupNo)
		selectableCount = uint32(len(pollOptionsBatch2))
		log.Printf("🗳️ Sending poll 2/%d with %d options (selectable: %d)...", numPolls, len(pollOptionsBatch2), selectableCount)
		success, statusMsg = ss.whatsAppSvc.SendPoll(groupConfig.JID, pollName, pollOptionsBatch2, selectableCount)
		if !success {
			return fmt.Errorf("failed to send poll 2/%d: %s", numPolls, statusMsg)
		}
		log.Printf("✅ Poll 2/%d sent successfully", numPolls)
	}

	// Update CSV with rotated adhyay numbers for next week
	log.Printf("📝 Updating CSV with rotated adhyay numbers...")
	if err := ss.updateCSVWithRotatedAdhyay(groupConfig.CSVPath, members, sevaType, groupNo); err != nil {
		log.Printf("⚠️ WARNING: Failed to update CSV: %v", err)
		// Don't halt the whole operation if CSV update fails
	} else {
		log.Printf("📝 CSV updated successfully")
	}

	log.Printf("✅ Seva automation completed for %s group %d", sevaType, groupNo)
	return nil
}

// updateCSVWithRotatedAdhyay updates the CSV file with rotated adhyay numbers
func (ss *SevaService) updateCSVWithRotatedAdhyay(csvPath string, members []domain.Member, sevaType domain.SevaType, groupNo int) error {
	// Create a new slice with rotated adhyay numbers
	maxAdhyay := ss.getMaxAdhyayForSeva(sevaType)
	updatedMembers := make([]domain.Member, len(members))
	for i, member := range members {
		updatedMembers[i] = member
		updatedMembers[i].AdhyayNo = member.AdhyayNo%maxAdhyay + 1
	}

	// Write updated members back to CSV
	return ss.csvRepo.WriteMembers(csvPath, updatedMembers, groupNo)
}

// getGroupConfig retrieves configuration for a specific group
func (ss *SevaService) getGroupConfig(sevaType domain.SevaType, groupNo int) (*domain.SevaGroup, error) {
	groups, ok := ss.config.Groups[string(sevaType)]
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

// buildSevaMessage creates the seva message based on seva type
func (ss *SevaService) buildSevaMessage(sevaType domain.SevaType, groupNo int, members []domain.Member) string {
	switch sevaType {
	case domain.SevaTypeMalhari:
		return ss.buildMalhariMessage(groupNo, members, sevaType)
	case domain.SevaTypeEkadashiBhagavat:
		return ss.buildEkadashiBhagavatMessage(groupNo, members, sevaType)
	case domain.SevaTypeSaptahikSwami:
		return ss.buildSaptahikSwamiMessage(groupNo, members, sevaType)
	case domain.SevaTypeDarbar:
		return ss.buildDarbarMessage(groupNo, members, sevaType)
	default:
		return ss.buildGenericMessage(groupNo, members, sevaType)
	}
}

// buildMalhariMessage creates the Malhari seva message
func (ss *SevaService) buildMalhariMessage(groupNo int, members []domain.Member, sevaType domain.SevaType) string {
	dayWithDate := getMarathiDayWithDate()
	// Build member list with rotated adhyay numbers
	memberList := ss.buildMemberListWithRotatedAdhyay(members, sevaType)
	message := fmt.Sprintf(`🙏 श्री स्वामी समर्थ🙏
श्री मल्हारी सप्तशती साप्ताहिक सेवा ग्रुप क्रमांक - %d
नमस्कार सर्वांना,🙏
आपण ही उपासना %s ला करायची आहे. आपल्या आई साहेबांची सेवा सुरू केलेली होती .म्हणून कुठेतरी खंत वाटत होती की आपण आपल्या वडिलांचे म्हणजे आपले कुलदैवत आपले महाराष्ट्राचे कुलदैवत श्री खंडेराव महाराज यांची सेवा करायला पाहिजे. म्हणून हा उपक्रम चालू केला. आपण सर्वांनी भरभरून प्रतिसाद दिला त्याबद्दल सर्वांचे या सेवेमध्ये खूप स्वागत आहे.🙏
आपला सेवेचा क्रम खालील प्रमाणे असणार आहे.
आधी गणपती बाप्पाचे स्मरण करून, आपल्या गुरूंचे स्मरण करून, आपल्याला सेवेला सुरुवात करायची आहे,
मल्हारी कवच,
मल्हारी रुदयस्तोत्र,
मल्हारी अष्टक,
मल्हारी मार्तंडांचा मंत्र जप 11 वेळा,
आणि तुमच्या नावापुढे जो क्रम राहील तो आध्यायक्रम राहील तो आध्याय तुम्ही वाचायचा. नंतर क्षमा प्रार्थना स्तोत्र म्हणुन आपले सेवा पूर्ण होईल.
आपली नावे🙏
%s
आपले नाव चेक करून घ्या. काही चुकले असल्यास मला लगेच कळवा. ही सेवा आपल्याला रविवारी पहाटे 5 ते संध्याकाळी 6 पर्यंत करायची आहे .आणि काही अडचण असल्यास ग्रुप वर एक दिवस आधीच मेसेज टाकणे अनिवार्य आहे. ग्रुपमधील ज्या सेवेकऱ्याला शक्य आहे त्यांनी ही सेवा करावी .सर्व ताईंना सांगणे आहे तुम्हाला अडचण असल्यास आपल्या पती किंवा मुलाकडून ही सेवा करून घेतलेली अत्यंत उत्तम राहणार आहे.
   संयोजक
मिनाक्षी बागुल 🙏`, groupNo, dayWithDate, memberList)

	//println(message)
	return message
}

// buildEkadashiBhagavatMessage creates the Ekadashi Bhagavat seva message
func (ss *SevaService) buildEkadashiBhagavatMessage(groupNo int, members []domain.Member, sevaType domain.SevaType) string {
	dayWithDate := getMarathiDayWithDate()
	message := fmt.Sprintf(`🙏 श्री स्वामी समर्थ 🙏नमस्कार सर्व सेवेकरीनां, 🙏गट - %d भागवत एकादशी सेवा
आपण फक्त एकादशीलाच हि सेवा करणार आहोत. अतिशय अनमोल अशी सेवा आपण सुरू करत आहोत. म्हणून शक्यतो कोणी काही अडचण जरी असले तरी घरातल्या व्यक्तींकडून सेवा करून घ्याल तर खूप छान राहील. 🙏
%s एकादशी आहे. या दिवशी आपण ही सेवा करणार आहोत.
आपला सेवेचा क्रम खालील प्रमाणे राहील.
श्री स्वामी स्तवन
श्री स्वामी समर्थ मंत्र 1 माळ जप
1 माळ विष्णू गायत्री मंत्र
1 माळ ओम नमो भगवते वासुदेवाय मंत्र
24 वेळा गायत्री मंत्र
24 वेळा लक्ष्मी गायत्री मंत्र
श्री गणपती अथर्वशीर्ष
नंतर आपण आपल्या नावासमोरील जो अंक येईल तो अध्याय क्रमांक खालील प्रमाणे वाचन करायचे आहे. 🙏
%s
हे आपले प्रत्येकाचे अध्यायक्रम आहेत. त्याप्रमाणे पहाटे 4 ते संध्याकाळी 4 वाजे पर्यंत आपल्याला ही सेवा करून ग्रुपवर मेसेज टाकणे. शक्यतो ही सेवा कोणीही कोणासाठी करणार नाही पण जास्तच अडचण असेल तर आदल्या दिवशी किंवा सकाळी दहाच्या आत सांगणे म्हणजे ग्रुप वरच एकमेकांना सहकार्य सेवा करता येईल नंतर सेवा होणार नाही. आम्ही संध्याकाळी चार नंतर डायरेक्ट तुमच्या नावाजवळच्या गोलला क्लिक करून देऊ तुम्ही सेवा केलेले असे समजून आम्ही तर निमित्त मात्र राहणार आहोत .पण आपण सर्व या सेवेअंतर्गत भगवंताशी जोडले गेलेलो आहोत .🙏🙏🙏
संयोजक
मिनाक्षी बागुल 🙏`, groupNo, dayWithDate, ss.buildMemberListWithRotatedAdhyay(members, sevaType))

	return message
}

// buildSaptahikSwamiMessage creates the Saptahik Swami seva message
func (ss *SevaService) buildSaptahikSwamiMessage(groupNo int, members []domain.Member, sevaType domain.SevaType) string {
	dayWithDate := getMarathiDayWithDate()
	memberList := ss.buildMemberListWithRotatedAdhyay(members, sevaType)
	message := fmt.Sprintf(`🙏 श्री स्वामी समर्थ🙏
श्री स्वामी चरित्र साप्ताहिक सेवा संघ क्रमांक - %d
ठरलेल्या नियमाप्रमाणे आपण ही सेवा केली तरच भक्तांच्या मनोकामना स्वामी नक्की पूर्ण करतील.
भिऊ नकोस मी तुझ्या पाठीशी आहे. हे आपल्या स्वामींचे ब्रीदवाक्यच आहे,
अशक्य ही शक्य करतील स्वामी.🙏
%s ला वाचायचे अध्याय खाली दिलेले आहेत. तुमच्या नावासमोर दिलेला अध्याय तुम्हाला वाचायचा आहे. व त्याचसोबत स्वामी प्रार्थना व श्री स्वामी समर्थ मंत्र जप ( १ ) माळ ही पण सेवा करायची आहे.
शेवटी आपल्याला तारक मंत्र म्हणायचं आहे.
पहाटे 5 ते सायंकाळी 6 वाजेपर्यंत  आपण ही सेवा लवकरात लवकर पूर्ण करून सायंकाळी. 6च्या आत मेसेज टाकून देणे अनिवार्य आहे.🙏
%s
प्रत्येकाने आपले नाव लगेच बघून घ्या .काही चुकलं असेल तर त्वरित मला फोन करावा. काही फारच अपरिहार्य कारणाने कोणी सेवा करू शकत नसेल तर आधी आपल्या घरातील कोणी व्यक्ती सेवा करत असेल तर त्यांच्याकडून करून घेणे नसेल  तर सकाळीच तसे ग्रुप वर कळवावे .ग्रुप मधील ज्यांना शक्य  असेल त्या सेविकाऱ्यांनी ही सेवा करावी.
   संयोजक 
मिनाक्षी बागुल 🙏`, groupNo, dayWithDate, memberList)
	return message
}

// buildDarbarMessage creates the Darbar seva message
func (ss *SevaService) buildDarbarMessage(groupNo int, members []domain.Member, sevaType domain.SevaType) string {
	dayWithDate := getMarathiDayWithDate()
	memberList := ss.buildMemberListWithRotatedAdhyay(members, sevaType)
	message := fmt.Sprintf(`🙏 श्री स्वामी समर्थ🙏
श्री दुर्गा सप्तशती साप्ताहिक सेवा दरबार क्रमांक - %d
ठरलेल्या नियमाप्रमाणे आपण ही उपासना केली तरच आईसाहेब आपल्या सर्वांच्या मनोकामना नक्कीच पूर्ण करतील खूप अनमोल सेवा आहे ही.🙏 
नमस्कार सखीनों , आपण ही, सेवा %s ला करायची आहे .प्रत्येकीने आधी गणपती बाप्पाचा स्मरण करून .मग आपल्या गुरूंचे स्मरण करून. स्वामी स्तवन किंवा प्रार्थना वाचावी,
देही सौभाग्य आरोग्य हे पाच मंत्र ,
देव्याकवचम ,
अर्गलास्तोत्रम् ,
अथकिलकस्तोत्रम ,
रात्री सुक्त
त्यानंतर आपल्या नावापुढील अध्याय वाचणे व (१) माळ नवार्णव मंत्र जप करून क्षमा प्रार्थना करणे.
प्रत्येकीने याप्रमाणे सेवा करायची आहे वेळ पहाटे 5 ते रात्री 6 वाजेपर्यंत सेवा करून ग्रुप वर मेसेज टाकणे अनिवार्य आहे.🙏
%s
ज्या सेवेकऱ्यांचा अध्याय तेरावा आहे, त्यांनी तेरावा अध्याय वाचल्या नंतर 
तंत्रोक्त देवीसूक्त,
प्राधानिक रहस्य,
विकृतिक रहस्य,
मूर्ती रहस्य,
आणि नंतर क्षमा प्रार्थना अशी सेवा आपल्याला करायची आहे . ज्यांचा तेरावा अध्याय आहे त्याच्या साठी सांगते आहे 

आपले नाव लगेच बघून घ्यावे काही चूक असल्यास मला त्वरित फोन करावा .काही फारच अपरिहार्य कारणाने कुणी सेवा करू शकत नसेल तर सकाळीचत तसे ग्रुप वर कळवावे .ग्रुप मध्ये ज्यांना शक्य असेल त्या सेवेकरींनी ही सेवा करावी.
   संयोजक 
मिनाक्षी बागुल 🙏`, groupNo, dayWithDate, memberList)
	return message
}

// (Add all your other build...Message and support/helper functions here from lines 320-477 of your codebase/screenshots)
// ...including buildDurgaPaathMessage, buildGenericMessage, and others as in code.

// buildGenericMessage creates a generic seva message (fallback)
func (ss *SevaService) buildGenericMessage(groupNo int, members []domain.Member, sevaType domain.SevaType) string {
	sevaName := getSevaDisplayName(sevaType)
	maxAdhyay := ss.getMaxAdhyayForSeva(sevaType)
	message := fmt.Sprintf(`*सर्व %s मंडळासाठी पुढची सेवा:*
(सूची - सेवा मंडळ सदस्य आणि त्यांचे अध्याय क्रमांक):

`, sevaName)

	// Create a slice listing all members with rotated adhyay
	type memberWithRotatedAdhyay struct {
		name       string
		nextAdhyay int
	}
	membersWithRotated := make([]memberWithRotatedAdhyay, 0, len(members))
	for _, member := range members {
		nextAdhyay := member.AdhyayNo%maxAdhyay + 1
		membersWithRotated = append(membersWithRotated, memberWithRotatedAdhyay{
			name:       member.Name,
			nextAdhyay: nextAdhyay,
		})
	}
	// Sort by rotated adhyay
	sort.Slice(membersWithRotated, func(i, j int) bool {
		return membersWithRotated[i].nextAdhyay < membersWithRotated[j].nextAdhyay
	})
	// List all members
	for _, m := range membersWithRotated {
		message += fmt.Sprintf("%s - अध्याय %d\n", m.name, m.nextAdhyay)
	}
	message += "\n🙏 सर्वांनी आपली सेवा वेळेवर पुर्ण करावी आणि Poll मध्ये हजेरी नोंदवावी अथवा ग्रुपवर सेवा पुर्ण झाल्याचे कळवावे. \n\n"
	message += "*जय सेवा* 🙏"
	return message
}

func getSevaDisplayName(sevaType domain.SevaType) any {
	switch sevaType {
	case domain.SevaTypeEkadashiBhagavat:
		return "एकादशी भागवत सेवा"
	case domain.SevaTypeDurgaPaath:
		return "दुर्गा पाठ सेवा"
	case domain.SevaTypeSaptahikSwami:
		return "साप्ताहिक स्वामी सेवा"
	case domain.SevaTypeDarbar:
		return "साप्ताहिक दुर्गा सेवा"
	case domain.SevaTypeMalhari:
		return "मल्हारी महात्म सेवा"
	default:
		return string(sevaType)
	}
}

// buildMemberListWithRotatedAdhyay creates a member list string with rotated adhyay numbers
// Members are sorted by their rotated adhyay number
func (ss *SevaService) buildMemberListWithRotatedAdhyay(members []domain.Member, sevaType domain.SevaType) string {
	maxAdhyay := ss.getMaxAdhyayForSeva(sevaType)
	type memberWithRotatedAdhyay struct {
		name       string
		nextAdhyay int
	}
	membersWithRotated := make([]memberWithRotatedAdhyay, 0, len(members))
	for _, member := range members {
		nextAdhyay := member.AdhyayNo%maxAdhyay + 1
		membersWithRotated = append(membersWithRotated, memberWithRotatedAdhyay{
			name:       member.Name,
			nextAdhyay: nextAdhyay,
		})
	}
	// Sort by rotated adhyay number
	sort.Slice(membersWithRotated, func(i, j int) bool {
		return membersWithRotated[i].nextAdhyay < membersWithRotated[j].nextAdhyay
	})
	// Build member list from sorted members
	memberList := ""
	for _, member := range membersWithRotated {
		memberList += fmt.Sprintf("%s - %d\n", member.name, member.nextAdhyay)
	}
	return memberList
}

// buildPollOptions creates poll options from members with rotated adhyay numbers
// Adds 1 to current adhyay, wraps around for maxAdhyay, and sorts by the new adhyay number
func (ss *SevaService) buildPollOptions(members []domain.Member, sevaType domain.SevaType) []string {
	type memberWithRotatedAdhyay struct {
		name       string
		nextAdhyay int
	}
	maxAdhyay := ss.getMaxAdhyayForSeva(sevaType)
	membersWithRotated := make([]memberWithRotatedAdhyay, 0, len(members))
	for _, member := range members {
		nextAdhyay := member.AdhyayNo%maxAdhyay + 1
		membersWithRotated = append(membersWithRotated, memberWithRotatedAdhyay{
			name:       member.Name,
			nextAdhyay: nextAdhyay,
		})
	}
	sort.Slice(membersWithRotated, func(i, j int) bool {
		return membersWithRotated[i].nextAdhyay < membersWithRotated[j].nextAdhyay
	})
	// Build poll options from sorted members
	options := make([]string, 0, len(membersWithRotated))
	for _, member := range membersWithRotated {
		option := fmt.Sprintf("%s - %d", member.name, member.nextAdhyay)
		options = append(options, option)
	}
	return options
}

// getMaxAdhyayForSeva returns the maximum adhyay number for a seva type
func (ss *SevaService) getMaxAdhyayForSeva(sevaType domain.SevaType) int {
	switch sevaType {
	case domain.SevaTypeMalhari, domain.SevaTypeEkadashiBhagavat:
		// Malhari & Ekadashi Saptashati and Ekadashi Bhagavat have 14 chapters
		return 14
	case domain.SevaTypeSaptahikSwami:
		// Swami Saptah weekly just use 1
		return 21
	case domain.SevaTypeDarbar:
		// Darbar generic seva (can be 13 or 14)
		return 13
	default:
		// Default to 13 for unknown seva types
		return 13
	}
}
