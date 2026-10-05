export async function request(url, options = {}, redirect = true) {
  const response = await fetch(url, { credentials: "same-origin", cache: "no-store", ...options });
  if (response.status === 401 && redirect) {
    location.assign("/file/signin");
    throw new Error("Your session has expired. Please sign in again.");
  }
  if (!response.ok) {
    const message = (await response.text()).trim();
    throw new Error(message || "The request failed. Please try again.");
  }
  return response;
}
