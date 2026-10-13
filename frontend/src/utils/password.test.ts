import { describe, expect, it } from "vitest";
import { passwordStrengthError, usernameError } from "./password";

describe("passwordStrengthError", () => {
  it("rejects short passwords", () => {
    expect(passwordStrengthError("Short1!")).not.toBeNull();
  });

  it("rejects passwords missing a character class", () => {
    expect(passwordStrengthError("alllowercase12345")).not.toBeNull();
    expect(passwordStrengthError("NoSymbolsOrDigitsHere")).not.toBeNull();
    expect(passwordStrengthError("NoDigitsHere!!!!")).not.toBeNull();
  });

  it("accepts a password with all four character classes at 12+ chars", () => {
    expect(passwordStrengthError("Valid-Password123!")).toBeNull();
  });
});

describe("usernameError", () => {
  it("accepts valid usernames", () => {
    for (const u of ["alice", "bob_2026", "user-01", "a23"]) {
      expect(usernameError(u)).toBeNull();
    }
  });

  it("rejects reserved usernames", () => {
    expect(usernameError("root")).not.toBeNull();
    expect(usernameError("postgres")).not.toBeNull();
  });

  it("rejects uppercase, spaces, and shell metacharacters", () => {
    expect(usernameError("Alice")).not.toBeNull();
    expect(usernameError("user name")).not.toBeNull();
    expect(usernameError("user;rm -rf /")).not.toBeNull();
  });

  it("rejects too-short and too-long usernames", () => {
    expect(usernameError("a")).not.toBeNull();
    expect(usernameError("this-username-is-way-too-long-to-be-valid")).not.toBeNull();
  });
});
