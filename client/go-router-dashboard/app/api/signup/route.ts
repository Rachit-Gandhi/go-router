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

  const backendResponse = await fetch(`${baseUrl}/users`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    cache: "no-store",
  });

  if (backendResponse.ok) {
    const data = await backendResponse.json();
    return NextResponse.json(data, { status: backendResponse.status });
  }

  const errorText = await backendResponse.text();
  return new NextResponse(errorText || "Request failed", {
    status: backendResponse.status,
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
