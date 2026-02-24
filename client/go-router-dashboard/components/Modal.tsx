"use client";

import { ReactNode } from "react";

type ModalProps = {
  open: boolean;
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
};

export function Modal({ open, title, subtitle, onClose, children }: ModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/55 p-4">
      <div className="w-full max-w-md rounded-2xl border border-border-color bg-card-background shadow-[0_20px_60px_rgba(0,0,0,0.22)]">
        <div className="px-5 pt-4">
          <div className="flex items-start justify-between">
            <div>
              <h2 className="text-xl font-semibold">{title}</h2>
              {subtitle ? (
                <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
              ) : null}
            </div>
            <button
              type="button"
              onClick={onClose}
              className="rounded-md px-2 py-1 text-muted-foreground hover:bg-[#f3f4fa] hover:text-foreground"
              aria-label="Close"
            >
              x
            </button>
          </div>
        </div>
        <div className="p-5 pt-4">{children}</div>
      </div>
    </div>
  );
}
