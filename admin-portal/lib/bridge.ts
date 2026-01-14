export type SevaType = string;

export type AdminGroup = {
  seva_type: string;
  number: number;
  jid: string;
  name: string;
  csv_path: string;
  max_adhyas: number;
  max_poll_size: number;
};

export type Member = {
  name: string;
  adhyay_no: number;
  phone_number?: string;
};

export type GroupMembersResponse = {
  seva_type: string;
  group_no: number;
  csv_path: string;
  hash: string;
  members: Member[];
};

export type GlobalMember = {
  key: string;
  name: string;
  phone_number?: string;
  groups: {
    seva_type: string;
    group_no: number;
    group_name: string;
    adhyay_no: number;
  }[];
};

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/bridge${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    },
    cache: "no-store"
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }

  return (await res.json()) as T;
}
