import { describe, expect, it } from "vitest";
import { daysUntil, formatBytes, formatUptime, isPast } from "./format";

describe("formatBytes", () => {
  it("handles null/undefined as unknown, not zero", () => {
    expect(formatBytes(null)).toBe("—");
    expect(formatBytes(undefined)).toBe("—");
  });

  it("formats zero explicitly", () => {
    expect(formatBytes(0)).toBe("0 B");
  });

  it("formats bytes, KB, MB, GB with sensible precision", () => {
    expect(formatBytes(500)).toBe("500 B");
    expect(formatBytes(2048)).toBe("2.00 KB");
    expect(formatBytes(5 * 1024 * 1024)).toBe("5.00 MB");
    expect(formatBytes(1024 * 1024 * 1024 * 3)).toBe("3.00 GB");
  });

  it("never fabricates a value for a negative input", () => {
    expect(formatBytes(-1)).toBe("—");
  });
});

describe("formatUptime", () => {
  it("formats days and hours", () => {
    expect(formatUptime(2 * 86400 + 3 * 3600)).toBe("2 يوم و3 ساعة");
  });

  it("formats minutes only when under an hour", () => {
    expect(formatUptime(5 * 60)).toBe("5 دقيقة");
  });

  it("falls back for sub-minute uptime", () => {
    expect(formatUptime(10)).toBe("أقل من دقيقة");
  });

  it("rejects invalid input rather than guessing", () => {
    expect(formatUptime(-5)).toBe("—");
    expect(formatUptime(NaN)).toBe("—");
  });
});

describe("isPast / daysUntil", () => {
  it("treats a far-future date as not past", () => {
    const future = new Date(Date.now() + 1000 * 60 * 60 * 24 * 30).toISOString();
    expect(isPast(future)).toBe(false);
    expect(daysUntil(future)).toBeGreaterThan(0);
  });

  it("treats a past date as past", () => {
    const past = new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString();
    expect(isPast(past)).toBe(true);
  });

  it("treats no expiry as not past", () => {
    expect(isPast(undefined)).toBe(false);
    expect(daysUntil(undefined)).toBeNull();
  });
});
