"use client";

import { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  loading?: boolean;
};

const variantClass: Record<Variant, string> = {
  primary:
    "bg-brand text-brand-foreground border border-brand shadow-[0_1px_0_rgba(255,255,255,0.2)_inset] hover:brightness-105",
  secondary:
    "bg-card-background text-foreground border border-border-color hover:bg-[#f4f5fb]",
  ghost:
    "bg-transparent text-muted-foreground border border-transparent hover:bg-[#f3f5ff] hover:text-foreground",
};

export function Button({
  variant = "primary",
  loading = false,
  className = "",
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center rounded-lg px-4 py-2.5 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60 ${variantClass[variant]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? "Please wait..." : children}
    </button>
  );
}
