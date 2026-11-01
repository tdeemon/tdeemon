import { cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { setLocale } from '@/i18n/locale'
import {
  fakeServer,
  problem,
  renderAt,
  session,
  signedIn,
  signedOut,
  type Handler,
} from '@/test/app'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  setLocale('en')
  localStorage.clear()
})

const token = 'made-up-invite-token'
const invited = 'iva@example.com'

// server is a server without a session whose invite of token checks,
// with a password and no provider unless over says otherwise; it keeps
// the bodies it was sent, by "METHOD /path". An account made starts the
// session the app asks for next.
function server(over: Record<string, Handler> = {}) {
  const sent: Record<string, unknown[]> = {}
  let live = false
  const keep = async (key: string, r: Request) => {
    ;(sent[key] ??= []).push(await r.clone().json())
  }
  fakeServer({
    ...signedIn(),
    ...signedOut,
    'GET /api/v1/auth/session': () =>
      live
        ? Response.json({ ...session(), user: { ...session().user, email: invited } })
        : problem(401),
    'GET /api/v1/auth/methods': () => Response.json({ password: true }),
    'POST /api/v1/invites/check': async (r) => {
      await keep('check', r)
      return Response.json({ email: invited, expires_at: '2026-10-30T10:00:00Z' })
    },
    'POST /api/v1/invites/accept': async (r) => {
      await keep('accept', r)
      live = true
      return Response.json(
        { ...session(), user: { ...session().user, email: invited } },
        { status: 201 },
      )
    },
    ...over,
  })
  return sent
}

// later is an answer a test gives when it likes.
function later() {
  let give: ((answer: Response) => void) | undefined
  const promise = new Promise<Response>((resolve) => {
    give = resolve
  })
  return {
    promise,
    resolve: (answer: Response) => {
      give?.(answer)
    },
  }
}

async function fill(name: string, password: string) {
  fireEvent.change(await screen.findByLabelText('Name'), { target: { value: name } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } })
  fireEvent.click(screen.getByRole('button', { name: 'Make the account' }))
}

test('an invite says the address, makes the account with a password, and lands on today', async () => {
  const sent = server()
  const router = renderAt(`/invite#${token}`)

  expect(await screen.findByRole('heading', { name: 'Make your account' })).toBeDefined()
  expect(await screen.findByText(`Your account will sign in as ${invited}.`)).toBeDefined()
  expect(screen.getByText(/^The invite works until /)).toBeDefined()
  expect(sent.check).toEqual([{ token }])
  // The password's least length is said by the field.
  const password = screen.getByLabelText('Password')
  expect(
    document.getElementById(password.getAttribute('aria-describedby') ?? '')?.textContent,
  ).toBe('At least 15 characters.')

  await fill(' Iva ', 'a long enough password')

  expect(await screen.findByRole('heading', { name: 'Today' })).toBeDefined()
  expect(router.state.location.pathname).toBe('/')
  expect(sent.accept).toEqual([
    {
      token,
      name: 'Iva',
      password: 'a long enough password',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      locale: 'en',
    },
  ])
})

test('a password too short or a name left out is said at its field, and not sent', async () => {
  const sent = server()
  renderAt(`/invite#${token}`)

  await fill('', 'short')

  const alerts = await screen.findAllByRole('alert')
  expect(alerts.map((a) => a.textContent)).toEqual(['A name is needed.', 'At least 15 characters.'])
  expect(screen.getByLabelText('Password').getAttribute('aria-invalid')).toBe('true')
  expect(sent.accept).toBeUndefined()
})

test('a link without a token, or one that does not work, says so with the way to sign in', async () => {
  server({ 'POST /api/v1/invites/check': () => problem(404) })
  renderAt(`/invite#${token}`)

  expect(
    await screen.findByText('This invite does not work: it expired or was used. Ask for another.'),
  ).toBeDefined()
  expect(screen.getByRole('link', { name: 'Go to sign in' }).getAttribute('href')).toBe('/login')
  expect(screen.queryByLabelText('Password')).toBeNull()
  cleanup()

  const sent = server()
  renderAt('/invite')
  expect(
    await screen.findByText('This invite does not work: it expired or was used. Ask for another.'),
  ).toBeDefined()
  expect(sent.check).toBeUndefined()
})

test.each([
  [404, {}, 'This invite does not work: it expired or was used. Ask for another.'],
  [409, {}, 'An account has this address already. Sign in instead.'],
  [422, {}, 'The server did not take it. Check your name and password.'],
  [503, { 'Retry-After': '5' }, 'The server is busy. Try again in 5 seconds.'],
  [500, {}, 'Making the account failed. Try again.'],
])('an account the server does not make (%i) says why', async (status, headers, said) => {
  server({ 'POST /api/v1/invites/accept': () => problem(status, headers) })
  const router = renderAt(`/invite#${token}`)

  await fill('Iva', 'a long enough password')

  expect((await screen.findByRole('alert')).textContent).toBe(said)
  expect(router.state.location.pathname).toBe('/invite')
})

test('an invite that cannot be checked says why, and is asked again', async () => {
  let fails = true
  server({
    'POST /api/v1/invites/check': () =>
      fails ? problem(500) : Response.json({ email: invited, expires_at: '2026-10-30T10:00:00Z' }),
  })
  renderAt(`/invite#${token}`)

  expect((await screen.findByRole('alert')).textContent).toBe('The server failed to answer.')
  fails = false
  fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
  expect(await screen.findByText(`Your account will sign in as ${invited}.`)).toBeDefined()
})

test("with a provider, the account is made through it, on the provider's page", async () => {
  const assign = vi.fn<(url: string) => void>()
  const { href, origin, pathname } = window.location
  vi.stubGlobal('location', { href, origin, pathname, hash: '', search: '', assign })
  const sent: unknown[] = []
  server({
    'GET /api/v1/auth/methods': () =>
      Response.json({ password: true, oidc: { name: 'Authentik', login_url: '/x' } }),
    'POST /api/v1/invites/sso': async (r) => {
      sent.push(await r.json())
      return Response.json({ url: 'https://id.example.com/authorize?state=x' })
    },
  })
  renderAt(`/invite#${token}`)

  fireEvent.click(await screen.findByRole('button', { name: 'Continue with Authentik' }))

  await waitFor(() => {
    expect(assign).toHaveBeenCalledWith('https://id.example.com/authorize?state=x')
  })
  expect(sent).toEqual([
    { token, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, locale: 'en' },
  ])
})

test('a link the provider cannot take says it does not work', async () => {
  server({
    'GET /api/v1/auth/methods': () =>
      Response.json({ password: true, oidc: { name: 'Authentik', login_url: '/x' } }),
    'POST /api/v1/invites/sso': () => problem(404),
  })
  renderAt(`/invite#${token}`)

  fireEvent.click(await screen.findByRole('button', { name: 'Continue with Authentik' }))

  expect((await screen.findByRole('alert')).textContent).toBe(
    'This invite does not work: it expired or was used. Ask for another.',
  )
})

test('while one way to make the account is on its way, neither is offered', async () => {
  const assign = vi.fn<(url: string) => void>()
  const { href, origin, pathname } = window.location
  vi.stubGlobal('location', { href, origin, pathname, hash: '', search: '', assign })
  // Neither answers until let go.
  const accepted = later()
  const started = later()
  const calls = { accept: 0, sso: 0 }
  server({
    'GET /api/v1/auth/methods': () =>
      Response.json({ password: true, oidc: { name: 'Authentik', login_url: '/x' } }),
    'POST /api/v1/invites/accept': () => {
      calls.accept++
      return accepted.promise
    },
    'POST /api/v1/invites/sso': () => {
      calls.sso++
      return started.promise
    },
  })
  renderAt(`/invite#${token}`)

  // The password's way first: the provider's waits.
  await fill('Iva', 'a long enough password')
  const provider = screen.getByRole('button', { name: 'Continue with Authentik' })
  await waitFor(() => {
    expect(provider.hasAttribute('disabled')).toBe(true)
  })
  fireEvent.click(provider)
  fireEvent.submit(screen.getByRole('button', { name: 'Making the account…' }))
  accepted.resolve(problem(500))
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Making the account failed. Try again.',
  )
  expect(provider.hasAttribute('disabled')).toBe(false)

  // Then the provider's: the form waits, and stays so once it answered.
  fireEvent.click(provider)
  const submit = screen.getByRole('button', { name: 'Make the account' })
  await waitFor(() => {
    expect(submit.hasAttribute('disabled')).toBe(true)
  })
  started.resolve(Response.json({ url: 'https://id.example.com/authorize' }))
  await waitFor(() => {
    expect(assign).toHaveBeenCalledTimes(1)
  })
  expect(submit.hasAttribute('disabled')).toBe(true)
  expect(provider.hasAttribute('disabled')).toBe(true)
  fireEvent.submit(submit)
  fireEvent.click(provider)
  expect(calls).toEqual({ accept: 1, sso: 1 })
})
