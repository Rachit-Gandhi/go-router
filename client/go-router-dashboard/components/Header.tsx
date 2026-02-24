"use client";

import { Button } from "@/components/Button";

type HeaderProps = {
  onSignupClick: () => void;
};

export function Header({ onSignupClick }: HeaderProps) {
  return (
    <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-5">
      <div className="text-lg font-semibold">GoRouter</div>
      <Button onClick={onSignupClick}>Sign up</Button>
    </header>
  );
}
