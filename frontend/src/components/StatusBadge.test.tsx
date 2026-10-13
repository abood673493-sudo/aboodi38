import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AccountStatusBadge, CertificateStatusBadge } from "./StatusBadge";

describe("AccountStatusBadge", () => {
  it("renders the Arabic label for each account status", () => {
    render(<AccountStatusBadge status="active" />);
    expect(screen.getByText("نشط")).toBeInTheDocument();
  });

  it("renders a distinct label for a disabled account", () => {
    render(<AccountStatusBadge status="disabled" />);
    expect(screen.getByText("معطّل")).toBeInTheDocument();
  });
});

describe("CertificateStatusBadge", () => {
  it("renders the Arabic label for an active certificate", () => {
    render(<CertificateStatusBadge status="active" />);
    expect(screen.getByText("فعّالة")).toBeInTheDocument();
  });

  it("renders the Arabic label for a failed certificate", () => {
    render(<CertificateStatusBadge status="failed" />);
    expect(screen.getByText("فشلت")).toBeInTheDocument();
  });
});
