import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, clearSession, getStoredSession, login, listAccounts } from "./client";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("api client", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });
  afterEach(() => {
    clearSession();
  });

  it("login() stores the returned session in localStorage", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse({
          access_token: "access-123",
          refresh_token: "refresh-456",
          username: "root",
          role: "admin",
        }),
      ),
    );

    const res = await login("root", "CorrectHorseBattery9!");

    expect(res.username).toBe("root");
    expect(getStoredSession().accessToken).toBe("access-123");
    expect(getStoredSession().refreshToken).toBe("refresh-456");
  });

  it("attaches the stored access token as a Bearer header on authenticated calls", async () => {
    localStorage.setItem("sshpanel_access_token", "my-token");
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    await listAccounts();

    const [, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = options.headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer my-token");
  });

  it("throws an ApiError carrying the backend's own error message on failure", async () => {
    localStorage.setItem("sshpanel_access_token", "my-token");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ error: "username already exists" }, 409)),
    );

    let caught: unknown;
    try {
      await listAccounts();
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(ApiError);
    expect((caught as ApiError).status).toBe(409);
    expect((caught as ApiError).message).toBe("username already exists");
  });

  it("treats an empty accounts list as an empty array, not undefined", async () => {
    localStorage.setItem("sshpanel_access_token", "my-token");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(null)));

    const accounts = await listAccounts();
    expect(accounts).toEqual([]);
  });
});
