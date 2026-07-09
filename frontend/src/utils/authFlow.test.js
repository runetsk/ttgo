import test from 'node:test';
import assert from 'node:assert/strict';
import { shouldRedirectToLogin } from './authFlow.js';

// An unauthenticated visitor is sent to the /login page (which shows sign-in, or
// the first-run "Create admin account" form on a brand-new instance). There is
// no in-place sign-in modal.
test('redirects an unauthenticated visitor to the login page', () => {
    assert.equal(
        shouldRedirectToLogin({ loading: false, user: null, pathname: '/library' }),
        true,
    );
});

// The gate is not library-specific — any protected route bounces to /login.
test('redirects from any protected route, not just the library', () => {
    assert.equal(
        shouldRedirectToLogin({ loading: false, user: null, pathname: '/runs/run/abc123' }),
        true,
    );
});

// Already on /login — redirecting again would loop.
test('does not redirect when already on the login page', () => {
    assert.equal(
        shouldRedirectToLogin({ loading: false, user: null, pathname: '/login' }),
        false,
    );
});

// A signed-in user stays where they are.
test('does not redirect an authenticated user', () => {
    assert.equal(
        shouldRedirectToLogin({ loading: false, user: { id: 'u1' }, pathname: '/library' }),
        false,
    );
});

// Auth state not resolved yet — don't bounce a user before we know they're out.
test('does not redirect while auth state is still loading', () => {
    assert.equal(
        shouldRedirectToLogin({ loading: true, user: null, pathname: '/library' }),
        false,
    );
});
