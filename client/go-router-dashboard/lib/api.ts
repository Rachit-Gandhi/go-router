export type SignupPayload = {
  username: string;
  email: string;
  password: string;
};

export type LoginPayload = {
  email: string;
  password: string;
};

export type ApiResult<T = string> = {
  ok: boolean;
  status: number;
  data?: T;
  error?: string;
};

export async function signup(payload: SignupPayload): Promise<ApiResult<{ message: string; username: string; email: string }>> {
  const response = await fetch("/api/signup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (response.ok) {
    const data = (await response.json()) as {
      message: string;
      username: string;
      email: string;
    };
    return { ok: true, status: response.status, data };
  }

  const error = await response.text();
  return { ok: false, status: response.status, error: error || "Unexpected error" };
}

export async function login(payload: LoginPayload): Promise<ApiResult<string>> {
  const response = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  const text = await response.text();
  if (response.ok) {
    return { ok: true, status: response.status, data: text || "Login successful" };
  }

  return { ok: false, status: response.status, error: text || "Unexpected error" };
}
