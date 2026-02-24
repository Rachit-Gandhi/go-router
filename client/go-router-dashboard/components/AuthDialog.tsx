"use client";

import { useState } from "react";
import { Button } from "@/components/Button";
import { AuthForm } from "@/components/AuthForm";
import { Modal } from "@/components/Modal";

type Mode = "signup" | "login";

type AuthDialogProps = {
  open: boolean;
  onClose: () => void;
};

export function AuthDialog({ open, onClose }: AuthDialogProps) {
  const [mode, setMode] = useState<Mode>("signup");

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "signup" ? "Create your account" : "Welcome back"}
      subtitle={
        mode === "signup"
          ? "Please fill in the details to get started."
          : "Sign in to continue."
      }
    >
      <div className="mb-3 grid grid-cols-3 gap-2">
        <button
          type="button"
          className="rounded-lg border border-border-color px-3 py-2 text-sm text-muted-foreground hover:bg-[#f5f6fc]"
        >
          GH
        </button>
        <button
          type="button"
          className="rounded-lg border border-border-color px-3 py-2 text-sm text-muted-foreground hover:bg-[#f5f6fc]"
        >
          G
        </button>
        <button
          type="button"
          className="rounded-lg border border-border-color px-3 py-2 text-sm text-muted-foreground hover:bg-[#f5f6fc]"
        >
          MM
        </button>
      </div>
      <div className="mb-4 flex items-center gap-3 text-xs text-muted-foreground">
        <div className="h-px flex-1 bg-border-color" />
        <span>or</span>
        <div className="h-px flex-1 bg-border-color" />
      </div>
      <div className="mb-4 grid grid-cols-2 gap-2 rounded-xl border border-border-color bg-[#fafbff] p-1">
        <Button
          type="button"
          variant={mode === "signup" ? "primary" : "ghost"}
          onClick={() => setMode("signup")}
          className="w-full"
        >
          Sign up
        </Button>
        <Button
          type="button"
          variant={mode === "login" ? "primary" : "ghost"}
          onClick={() => setMode("login")}
          className="w-full"
        >
          Log in
        </Button>
      </div>
      <AuthForm mode={mode} onSwitchMode={setMode} onSuccess={onClose} />
    </Modal>
  );
}
