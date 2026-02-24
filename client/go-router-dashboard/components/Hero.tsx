"use client";

import { Button } from "@/components/Button";

type HeroProps = {
  onSignupClick: () => void;
};

export function Hero({ onSignupClick }: HeroProps) {
  return (
    <section className="mx-auto mt-14 flex w-full max-w-4xl flex-col items-center px-6 text-center">
      <h1 className="max-w-3xl text-5xl font-semibold tracking-tight md:text-6xl">
        The Unified Interface For LLMs
      </h1>
      <p className="mt-5 text-xl text-muted-foreground">
        Better prices, better uptime, no subscriptions.
      </p>
      <div className="mt-10">
        <Button className="min-w-44" onClick={onSignupClick}>
          Get API Key
        </Button>
      </div>
    </section>
  );
}
