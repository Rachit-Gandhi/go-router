"use client";

import { InputHTMLAttributes } from "react";

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string;
};

export function Input({ label, className = "", ...props }: InputProps) {
  return (
    <label className="flex flex-col gap-1.5 text-sm">
      <span className="text-[13px] font-medium text-foreground/85">{label}</span>
      <input
        className={`rounded-lg border border-border-color bg-card-background px-3 py-2.5 text-sm outline-none transition placeholder:text-muted-foreground/70 focus:border-brand focus:ring-2 focus:ring-brand/15 ${className}`}
        {...props}
      />
    </label>
  );
}
