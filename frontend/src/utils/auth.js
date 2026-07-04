import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants";
import { getApiPath } from "@/utils/url.js";

export async function validateLogin(isPublicRoute = false) {
  // Use direct fetch to avoid automatic logout on 401
  // Public routes (e.g. /public/share/...) use the public API base path
  const apiPath = getApiPath('users', { id: 'self' }, false, isPublicRoute);
  const res = await fetch(apiPath, {
    credentials: 'same-origin', // Ensure cookies are sent with the request
    headers: {
      "sessionId": state.sessionId,
    }
  });

  if (res.status === 401) {
    // Session expired — try to refresh using remember-me token
    const refreshed = await refresh();
    if (refreshed) {
      // Retry the original request with the new session
      const retryRes = await fetch(apiPath, {
        credentials: 'same-origin',
        headers: {
          "sessionId": state.sessionId,
        }
      });
      if (retryRes.status === 200) {
        const userInfo = await retryRes.json();
        mutations.setCurrentUser(userInfo);
        getters.isLoggedIn();
        return;
      }
    }
    throw new Error(`{"status":401,"message":"session expired"}`);
  }

  if (res.status !== 200) {
    throw new Error(`{"status":${res.status},"message":"${await res.text()}"}`);
  }
  const userInfo = await res.json();
  await mutations.setCurrentUser(userInfo);
  getters.isLoggedIn()
  if (state.user.loginMethod === "proxy") {
    const apiPath = getApiPath("auth/login")
    const res = await fetch(apiPath, {
      method: "POST",
      credentials: 'same-origin', // Ensure cookies are sent and can be set
    });
    const body = await res.text();
    if (res.status !== 200) {
      throw new Error(body);
    }
  }
  return
}

export async function renew() {
  // Cookie-based renewal - no JWT parameter needed
  // Backend reads cookie, validates, and sets new cookie
  const apiPath = getApiPath("auth/renew")
  const res = await fetch(apiPath, {
    method: "POST",
    credentials: 'same-origin', // Cookie is sent automatically, backend renews it
  });
  const body = await res.text();
  if (res.status === 200) {
    mutations.setSession(generateRandomCode(8));
    // Backend sets the new cookie, no state management needed
  } else {
    throw new Error(body);
  }
}

// refresh uses the long-lived refresh token cookie to get a new session.
// Returns true if refresh was successful, false otherwise.
export async function refresh() {
  try {
    const apiPath = getApiPath("auth/refresh");
    const res = await fetch(apiPath, {
      method: "POST",
      credentials: 'same-origin', // Refresh cookie is sent automatically (HTTP-only)
    });
    if (res.status === 200) {
      mutations.setSession(generateRandomCode(8));
      return true;
    }
    return false;
  } catch {
    return false;
  }
}

export function generateRandomCode(length) {
  const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let code = '';
  for (let i = 0; i < length; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length);
    code += charset.charAt(randomIndex);
  }

  return code;
}

export async function logout() {
  try {
    const res = await fetch(getApiPath("auth/logout"), {
      method: "POST",
      credentials: 'same-origin'
    });
    if (res.ok) {
      const data = await res.json();
      let logoutUrl = data.logoutUrl;
      // Backend clears the cookie, but frontend does it as fail-safe cleanup
      document.cookie = "filebrowser_quantum_jwt=; expires=Thu, 01 Jan 1970 00:00:01 GMT; path=/";
      void mutations.setCurrentUser(null);
      // No need to clear state.jwt - cookie is the source of truth
      if (!logoutUrl) {
        logoutUrl = `${globalVars.baseURL}login`;
      }
      // Add a small delay to ensure cookie deletion completes before redirect
      setTimeout(() => {
        window.location.href = logoutUrl;
      }, 100);
      return; // Stop execution
    } else {
      // Handle potential errors from the API, e.g., res.status 401, 500
      console.error("Logout API call failed:", res.status, res.statusText);
    }
  } catch (e) {
    console.error("An error occurred during logout:", e);
  }
}

// Helper function to retrieve the value of a specific cookie
//function getCookie(name) {
//  return document.cookie
//    .split('; ')
//    .find(row => row.startsWith(name + '='))
//    ?.split('=')[1];
//}

export async function initAuth() {
  if (!getters.isShare()) {
    await validateLogin();
  }
  if (globalVars.recaptcha) {
      await new Promise((resolve) => {
          const check = () => {
              if (typeof window.grecaptcha === "undefined") {
                  setTimeout(check, 100);
              } else {
                  resolve();
              }
          };
          check();
      });
  }
}
