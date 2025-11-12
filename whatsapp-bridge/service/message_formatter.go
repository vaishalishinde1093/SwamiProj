package service

import (
	"fmt"
	"strings"
	"time"
	"whatsapp-client/domain"
)

// formatTomorrowDateMarathi returns tomorrow's date formatted in Marathi with day of week
// Format: "DD Month YYYY Weekday" (e.g., "23 ऑक्टोबर 2025 गुरुवार")
func formatTomorrowDateMarathi() string {
	tomorrow := time.Now().AddDate(0, 0, 1)
	marathiMonths := []string{
		"जानेवारी", "फेब्रुवारी", "मार्च", "एप्रिल", "मे", "जून",
		"जुलै", "ऑगस्ट", "सप्टेंबर", "ऑक्टोबर", "नोव्हेंबर", "डिसेंबर",
	}
	marathiWeekdays := []string{
		"रविवार",   // Sunday
		"सोमवार",   // Monday
		"मंगळवार",  // Tuesday
		"बुधवार",   // Wednesday
		"गुरुवार",  // Thursday
		"शुक्रवार", // Friday
		"शनिवार",   // Saturday
	}
	dayName := marathiWeekdays[tomorrow.Weekday()]
	monthName := marathiMonths[tomorrow.Month()-1]
	return fmt.Sprintf("%d %s %d %s", tomorrow.Day(), monthName, tomorrow.Year(), dayName)
}

// FormatSevaMessageForGroup generates the formatted message content for a given seva group.
// This function preserves the ORIGINAL message templates used by Amee Saheb.
// Do not modify message templates without approval!
//
// Always use tomorrow's seva date in Marathi format if nextSevaDate is empty
func FormatSevaMessageForGroup(sevaType domain.SevaType, groupNum int, members []domain.Member, nextSevaDate string) string {
	if nextSevaDate == "" {
		nextSevaDate = formatTomorrowDateMarathi()
	}

	// Format member list
	var memberList strings.Builder
	for _, member := range members {
		memberList.WriteString(fmt.Sprintf("%s (%d), ", member.Name, member.AdhyayNo))
	}

	switch sevaType {
	case domain.SevaTypeEkadashiBhagavat:
		// ORIGINAL template from ekadashi_bhagavat_seva.go Line 225-240
		return fmt.Sprintf(`🙏 नमस्कार मंडळी,
उद्या, %s रोजी मंडळ %d, श्रीमद् भागवत, अ. १  वाचे त्यांनी खालीलप्रमाणे सेवा करायची आहे:
%s
कृपया सर्वांनी गुरुजीच्या मार्गदर्शनानुसार सेवा पुर्ण करावी व सेवा पुर्ण झाल्यावर पुढील फॉर्म भरावा: [सेवा अहवाल फॉर्म लिंक].
🙏🙏`, nextSevaDate, groupNum, memberList.String())

	case domain.SevaTypeDurgaPaath:
		// ORIGINAL template from main.go (Line 3627-3641)
		return fmt.Sprintf(`🙏 नमस्कार,
%d नंबरच्या ग्रुपने उद्या %s रोजी दुर्गा पाठ सेवा करायची आहे:
%s
जो कोणी सेवा करेल त्यांनी सेवा पुर्ण केल्यावर ही लिंक भरा: [सेवा अहवाल फॉर्म लिंक].
🙏`, groupNum, nextSevaDate, memberList.String())

	case domain.SevaTypeMalhari:
		// ORIGINAL template from main.go (Line 4248-4259)
		// Note: Saptah & Malhari use suffix on the date
		return fmt.Sprintf(`🙏 श्री स्वामी समर्थ🙏
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
मिनाक्षी बागुल 🙏`, groupNum, nextSevaDate, memberList.String())

	case domain.SevaTypeSaptahikSwami:
		// ORIGINAL template from main.go (Line 3457-3479)
		return fmt.Sprintf(`🙏 नमस्कार मंडळी,
मंडळ %d यांनी उद्या %s रोजी साप्ताहिक स्वामी सेवा (स्वामी समर्थ आरती, स्तोत्र, पवित्रा, धूप, आरती, नामस्मरण) घ्यावी अशी विनंती आहे.
%s
कृपया सेवा पुर्ण केल्यावर अहवाल फॉर्म भरा: [सेवा अहवाल फॉर्म लिंक].
🙏`, groupNum, nextSevaDate, memberList.String())

	case domain.SevaTypeDarbar:
		// ORIGINAL template from main.go (Line 3827-3841)
		// Template in Marathi, as per code on the date
		message := fmt.Sprintf(`🙏 नमस्कार मंडळी,

मंडळ %d ची पुढील ग्रुप सेवा %s रोजी आहे.
%s

कृपया सर्वांनी सेवा पुर्ण करावी व सेवा पुर्ण झाल्यावर अहवाल भरा: [सेवा अहवाल फॉर्म लिंक].
🙏🙏`, groupNum, nextSevaDate, memberList.String())
		return message

	default:
		return fmt.Sprintf("Unknown Seva Type: %s for Group %d on %s; %s", sevaType, groupNum, nextSevaDate, memberList.String())
	}
}
