// Session-cookie auth for WebSocket load (S4). Bearer tokens CANNOT open a
// WS connection — the upgrade handler validates only the session_token
// cookie (backend/internal/api/websocket/ws_handler.go) — so S4 logs seeded
// perf users in and hands each VU a cookie. See
// docs/superpowers/plans/2026-07-05-s4-ws-investigation.md for the full map.
import http from 'k6/http';
import { sleep } from 'k6';
import { SharedArray } from 'k6/data';
import { BASE_URL, MANIFEST_PATH } from './api.js';

const CONNS_PER_USER = 20; // server cap, keyed on User.ID (ws_client.go)

const userPool = new SharedArray('perf ws users', () => {
  const m = JSON.parse(open(MANIFEST_PATH));
  const emails = m.user_emails || [];
  const password = m.user_password || '';
  if (emails.length === 0 || password === '') {
    throw new Error(
      'seed manifest has no user_emails/user_password — reseed with RESEED=1 (older manifests predate user_password)'
    );
  }
  // SharedArray must hold plain elements; pack password with each email.
  return emails.map((e) => `${e}\n${password}`);
});

// Init-context cap check: clients beyond users*20 would be rejected by the
// per-user cap mid-ramp — abort before any load instead.
export function usersForClients(clients) {
  const available = userPool.length;
  const needed = Math.ceil(clients / CONNS_PER_USER);
  if (needed > available) {
    throw new Error(
      `${clients} WS clients need ${needed} users (20-conn/user cap) but the manifest has ${available} — reseed with USERS=${needed} RESEED=1`
    );
  }
  const emails = [];
  for (let i = 0; i < needed; i++) {
    emails.push(userPool[i].split('\n')[0]);
  }
  return { emails, password: userPool[0].split('\n')[1] };
}

// Call from setup() only. Paces logins under the per-IP limiter
// (ratelimit.New(0.5, 10): burst 10, then 1 login per 2s) — 50 users ≈ 90s,
// so the scenario must set setupTimeout accordingly.
export function loginAll(emails, password) {
  const cookies = [];
  for (let i = 0; i < emails.length; i++) {
    if (i >= 10) {
      sleep(2.1);
    }
    const res = http.post(
      `${BASE_URL}/api/auth/login`,
      JSON.stringify({ email: emails[i], password }),
      { headers: { 'Content-Type': 'application/json' } }
    );
    const jar = res.cookies['session_token'];
    if (res.status !== 200 || !jar || !jar.length) {
      throw new Error(`login failed for ${emails[i]}: HTTP ${res.status}`);
    }
    cookies.push(jar[0].value);
  }
  return cookies;
}

// ws.connect params: the handler requires the session cookie AND a
// non-empty Origin matching its allow-list or the request Host.
export function wsParams(cookie) {
  return { headers: { Cookie: `session_token=${cookie}`, Origin: BASE_URL } };
}
