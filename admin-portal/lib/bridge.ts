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
  version: number;
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

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

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
    throw new ApiError(text || `Request failed: ${res.status}`, res.status);
  }

  return (await res.json()) as T;
}
