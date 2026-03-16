import { NextRequest, NextResponse } from "next/server";

const HOP_BY_HOP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
  "host",
  "content-length"
]);

function sanitizeOutgoingHeaders(source: Headers): Headers {
  const headers = new Headers();
  source.forEach((value, key) => {
    if (!HOP_BY_HOP_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  });
  return headers;
}

function sanitizeIncomingHeaders(source: Headers): Headers {
  const headers = new Headers();
  source.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (!HOP_BY_HOP_HEADERS.has(lower) && lower !== "set-cookie") {
      headers.append(key, value);
    }
  });

  const withSetCookie = source as Headers & { getSetCookie?: () => string[] };
  if (typeof withSetCookie.getSetCookie === "function") {
    for (const cookie of withSetCookie.getSetCookie()) {
      headers.append("set-cookie", cookie);
    }
  } else {
    const cookie = source.get("set-cookie");
    if (cookie) {
      headers.append("set-cookie", cookie);
    }
  }

  return headers;
}

function normalizeBase(baseUrl: string): string {
  return baseUrl.endsWith("/") ? baseUrl.slice(0, -1) : baseUrl;
}

export async function proxyRequest(
  request: NextRequest,
  baseUrl: string,
  pathSegments: string[]
): Promise<NextResponse> {
  let targetUrl = "";
  try {
    const path = pathSegments.map((segment) => encodeURIComponent(segment)).join("/");
    targetUrl = `${normalizeBase(baseUrl)}/${path}${request.nextUrl.search}`;
    const outgoingHeaders = sanitizeOutgoingHeaders(request.headers);

    const init: RequestInit = {
      method: request.method,
      headers: outgoingHeaders,
      cache: "no-store",
      redirect: "manual"
    };

    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = await request.arrayBuffer();
    }

    const upstream = await fetch(targetUrl, init);
    const incomingHeaders = sanitizeIncomingHeaders(upstream.headers);

    if (upstream.body) {
      return new NextResponse(upstream.body, {
        status: upstream.status,
        headers: incomingHeaders
      });
    }

    const body = await upstream.arrayBuffer();
    return new NextResponse(body, {
      status: upstream.status,
      headers: incomingHeaders
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "proxy request failed";
    console.error("[proxy] upstream request failed", {
      message,
      method: request.method,
      requestUrl: request.nextUrl.toString(),
      targetUrl,
      error
    });
    return NextResponse.json(
      {
        error: message
      },
      {
        status: 502
      }
    );
  }
}
