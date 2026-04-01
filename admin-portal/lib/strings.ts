export const mr = {
  app: {
    nameTop: "श्री स्वामी समर्थ",
    name: "सेवा व्यवस्थापन",
    poweredBy: "दिंडोरी प्रणित सेवा मार्ग"
  },
  nav: {
    dashboard: "डॅशबोर्ड",
    groups: "गट",
    members: "सदस्य"
  },
  home: {
    kicker: "प्रशासन",
    title: "सेवा व्यवस्थापन पोर्टल",
    subtitle:
      "गट संरचना, सदस्य यादी (CSV आधारित), आणि सेवा क्रिया (सेवा पाठवा, पर्सनल नोटिफिकेशन पाठवा, ग्रुप वर राहिलेले सेवेकरी नोंदवा) एकाच ठिकाणाहून व्यवस्थापित करा.",
    openDashboard: "डॅशबोर्ड उघडा",
    cards: {
      operateSevas: {
        title: "सेवा संचालन",
        description: "सेवा पाठवा, पर्सनल नोटिफिकेशन पाठवा, आणि ग्रुप वर राहिलेले सेवेकरी नोंदवा."
      },
      manageGroups: {
        title: "गट व्यवस्थापन",
        description: "गटाचे नाव/JID/CSV पाथ/पोल मर्यादा संपादित करा आणि groups.yaml मध्ये जतन करा."
      },
      membersDirectory: {
        title: "सदस्य निर्देशिका",
        description: "सर्व गटांतील सदस्यांची एकत्रित यादी आणि गट-निहाय नेव्हिगेशन."
      }
    }
  },
  dashboard: {
    kicker: "संचालन",
    title: "सेवा डॅशबोर्ड",
    subtitle: "गटानुसार क्रिया जलद आणि सुरक्षितपणे चालवा.",
    manageGroups: "गट व्यवस्थापित करा",
    loadingGroups: "गट लोड होत आहेत…",
    noEndpoint: "या सेवा प्रकारासाठी endpoint mapping उपलब्ध नाही.",
    actionDone: "क्रिया पूर्ण झाली.",
    actionFailed: "क्रिया अयशस्वी",
    sevaType: "सेवा प्रकार",
    group: "गट",
    jid: "JID",
    pollMessage: "सेवा पाठवा",
    remind: "पर्सनल नोटिफिकेशन पाठवा",
    announce: "ग्रुप वर राहिलेले सेवेकरी नोंदवा",
    editMembers: "सदस्य संपादित करा",
    sendSevaToAll: "सर्व गटांना सेवा पाठवा",
    remindAll: "सर्वांना पर्सनल नोटिफिकेशन पाठवा",
    announceToAll: "सर्व गटांवर राहिलेले सेवेकरी नोंदवा"
  },
  groups: {
    kicker: "संरचना",
    title: "गट",
    subtitlePrefix: "बदल जतन होतील:",
    reload: "रीलोड",
    refresh: "रिफ्रेश",
    loading: "लोड होत आहे…",
    reloaded: "डिस्कवरून कॉन्फिग रीलोड झाले.",
    reloadFailed: "रीलोड अयशस्वी",
    failedToLoad: "लोड अयशस्वी",
    save: "जतन करा",
    saveFailed: "जतन अयशस्वी",
    savedPrefix: "जतन झाले:",
    sevaType: "सेवा प्रकार",
    group: "गट",
    editMembers: "सदस्य संपादित करा",
    fields: {
      name: "नाव",
      jid: "JID",
      csvPath: "CSV पाथ",
      maxAdhyas: "कमाल अध्याय",
      maxPollSize: "कमाल पोल आकार"
    }
  },
  groupMembers: {
    back: "मागे",
    titlePrefix: "सदस्य",
    total: "एकूण",
    withPhone: "फोनसह",
    csv: "CSV",
    add: "जोडा",
    save: "जतन करा",
    saved: "जतन झाले.",
    loading: "लोड होत आहे…",
    failedToLoad: "लोड अयशस्वी",
    saveFailed: "जतन अयशस्वी",
    headers: {
      name: "नाव",
      adhyay: "अध्याय",
      phone: "फोन"
    },
    placeholders: {
      memberName: "सदस्याचे नाव",
      phoneOptional: "ऐच्छिक फोन (पर्सनल नोटिफिकेशनसाठी)"
    },
    remove: "काढा",
    empty: "या गटात कोणतेही सदस्य नाहीत."
  },
  members: {
    kicker: "निर्देशिका",
    title: "सदस्य",
    subtitle: "सर्व गटांच्या CSV फाइल्समधून एकत्रित.",
    loading: "लोड होत आहे…",
    failedToLoad: "लोड अयशस्वी",
    searchPlaceholder: "नाव किंवा फोनने शोधा",
    showingPrefix: "दाखवत आहे",
    of: "पैकी",
    noName: "(नाव नाही)",
    noPhone: "फोन नाही",
    noMatches: "जुळणारे निकाल नाहीत."
  }
} as const;

export type Strings = typeof mr;

export function t(path: string): string {
  const parts = path.split(".");
  let cur: any = mr;
  for (const p of parts) {
    cur = cur?.[p];
  }
  return typeof cur === "string" ? cur : path;
}

export function sevaTypeLabel(sevaType: string): string {
  switch (sevaType) {
    case "ekadashi_bhagavat":
      return "एकादशी भागवत";
    case "durga_paath":
      return "दुर्गा पाठ";
    case "saptahik_swami":
      return "साप्ताहिक स्वामी सेवा";
    case "malhari":
      return "मल्हारी";
    case "darbar":
      return "दरबार";
    case "chaitra_navratri":
      return "चैत्र नवरात्र पाठ";
    default:
      return sevaType;
  }
}
