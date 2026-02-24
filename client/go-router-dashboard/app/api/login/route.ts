import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest) {
  const baseUrl = process.env.CONTROL_API_BASE_URL;
  if (!baseUrl) {
    return new NextResponse("CONTROL_API_BASE_URL is not configured", {
      status: 500,
    });
  }

  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return new NextResponse("Failed to parse request body", { status: 400 });
  }

  const backendResponse = await fetch(`${baseUrl}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    cache: "no-store",
  });

  const text = await backendResponse.text();
  const headers = new Headers({ "Content-Type": "text/plain; charset=utf-8" });
  const setCookie = backendResponse.headers.get("set-cookie");
  if (setCookie) {
    headers.set("set-cookie", setCookie);
  }

  return new NextResponse(text, {
    status: backendResponse.status,
    headers,
  });
}
