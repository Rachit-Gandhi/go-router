"use client";

import { FormEvent, useState } from "react";
import { Button } from "@/components/Button";
import { Input } from "@/components/Input";
import { login, signup } from "@/lib/api";

type Mode = "signup" | "login";

type AuthFormProps = {
  mode: Mode;
  onSwitchMode: (mode: Mode) => void;
  onSuccess?: () => void;
};

export function AuthForm({ mode, onSwitchMode, onSuccess }: AuthFormProps) {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [acceptedTerms, setAcceptedTerms] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");

    if (mode === "signup" && !username.trim()) {
      setError("username is required");
      return;
    }
    if (!email.trim()) {
      setError("email is required");
      return;
    }
    if (!password.trim()) {
      setError("password is required");
      return;
    }
    if (mode === "signup" && !acceptedTerms) {
      setError("please accept terms to continue");
      return;
    }

    setSubmitting(true);
    try {
      if (mode === "signup") {
        const result = await signup({ username, email, password });
        if (!result.ok) {
          setError(result.error ?? "Unexpected error");
          return;
        }
        setSuccess(result.data?.message ?? "User created successfully");
        onSwitchMode("login");
      } else {
        const result = await login({ email, password });
        if (!result.ok) {
          setError(result.error ?? "Unexpected error");
          return;
        }
        setSuccess(result.data ?? "Login successful");
        if (onSuccess) {
          setTimeout(() => onSuccess(), 500);
        }
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      {mode === "signup" ? (
        <Input
          label="Username"
          type="text"
          value={username}
          onChange={(event) => setUsername(event.target.value)}
          placeholder="Enter your username"
          autoComplete="username"
        />
      ) : null}
      <Input
        label="Email address"
        type="email"
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        placeholder="Enter your email address"
        autoComplete="email"
      />
      <Input
        label="Password"
        type="password"
        value={password}
        onChange={(event) => setPassword(event.target.value)}
        placeholder="Enter your password"
        autoComplete={mode === "signup" ? "new-password" : "current-password"}
      />
      {mode === "signup" ? (
        <label className="flex items-start gap-2 text-xs text-muted-foreground">
          <input
            type="checkbox"
            checked={acceptedTerms}
            onChange={(event) => setAcceptedTerms(event.target.checked)}
            className="mt-0.5 h-3.5 w-3.5 rounded border-border-color"
          />
          <span>
            I agree to the <span className="underline">Terms of Service</span> and{" "}
            <span className="underline">Privacy Policy</span>
          </span>
        </label>
      ) : null}
      {error ? (
        <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </div>
      ) : null}
      {success ? (
        <div className="rounded-lg border border-green-200 bg-green-50 px-3 py-2 text-sm text-green-700">
          {success}
        </div>
      ) : null}
      <Button type="submit" className="w-full" loading={submitting}>
        {mode === "signup" ? "Continue" : "Log in"}
      </Button>
      <div className="mt-3 border-t border-border-color pt-3 text-center text-sm text-muted-foreground">
        {mode === "signup" ? "Already have an account?" : "Need an account?"}{" "}
        <button
          type="button"
          className="font-medium text-brand hover:underline"
          onClick={() => onSwitchMode(mode === "signup" ? "login" : "signup")}
        >
          {mode === "signup" ? "Sign in" : "Sign up"}
        </button>
      </div>
    </form>
  );
}
