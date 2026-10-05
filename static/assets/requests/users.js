import { request } from "./http.js?v=14";

export async function getUser(redirect = true) {
  return (await request("/api/users/me", {}, redirect)).json();
}
export function authenticate(url, body) {
  return request(url, { method: "POST", body }, false);
}
export function signOut(url) {
  return request(url, { method: "POST" }, false);
}
export function updateUser(kind, body) {
  return request(`/api/users/me/${kind}`, { method: "PATCH", body }, kind !== "password");
}
