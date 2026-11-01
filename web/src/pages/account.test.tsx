import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { setLocale } from '@/i18n/locale'
import { fakeServer, problem, renderAt, session, signedIn, type Handler } from '@/test/app'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  setLocale('en')
  localStorage.clear()
})

// server is a signed-in server whose account is deleted by deleted, and
// which is signed out once it is; it keeps the bodies of the deletes.
function server(
  deleted: Handler = () => new Response(null, { status: 204 }),
  email = session().user.email,
) {
  const sent: unknown[] = []
  let live = true
  fakeServer({
    ...signedIn(),
    'GET /api/v1/auth/session': () =>
      live ? Response.json({ ...session(), user: { ...session().user, email } }) : problem(401),
    'GET /api/v1/auth/methods': () => Response.json({ password: true }),
    'DELETE /api/v1/account': async (r) => {
      sent.push(await r.clone().json())
      const answer = await deleted(r)
      live = !answer.ok
      return answer
    },
  })
  return sent
}

async function yourData() {
  return within(await screen.findByRole('region', { name: 'Your data' }))
}

test('the account page exports the data as a download', async () => {
  server()
  renderAt('/settings/account')

  const data = await yourData()
  const link = data.getByRole('link', { name: 'Export your data' })
  expect(link.getAttribute('href')).toBe('/api/v1/account/export')
  expect(link.hasAttribute('download')).toBe(true)
})

test('the account is deleted once its address is typed, and the browser is at sign-in', async () => {
  const sent = server()
  const router = renderAt('/settings/account')

  fireEvent.click(await (await yourData()).findByRole('button', { name: 'Delete your account' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'Delete your account' }))
  expect(sheet.getByText(/for good: meals, foods/)).toBeDefined()
  expect(sheet.getByText(/snapshots keep them/)).toBeDefined()
  const field = sheet.getByLabelText('Your email address')
  await waitFor(() => {
    expect(document.activeElement).toBe(field)
  })
  const button = sheet.getByRole('button', { name: 'Delete my account' })
  expect(button.hasAttribute('disabled')).toBe(true)
  fireEvent.change(field, { target: { value: 'ana@example' } })
  expect(button.hasAttribute('disabled')).toBe(true)
  fireEvent.change(field, { target: { value: 'Ana@Example.com ' } })
  expect(button.hasAttribute('disabled')).toBe(false)
  fireEvent.click(button)

  expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeDefined()
  expect(router.state.location.pathname).toBe('/login')
  expect(sent).toEqual([{ email: 'Ana@Example.com' }])
})

test.each([
  [409, 'You are the only admin. Make another user an admin first.'],
  [422, "That is not your account's email address."],
  [500, 'Your account was not deleted. The server failed to answer.'],
])('an account the server does not delete (%i) says why', async (status, said) => {
  server(() => problem(status))
  const router = renderAt('/settings/account')

  fireEvent.click(await (await yourData()).findByRole('button', { name: 'Delete your account' }))
  const sheet = within(await screen.findByRole('dialog'))
  fireEvent.change(sheet.getByLabelText('Your email address'), {
    target: { value: 'ana@example.com' },
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Delete my account' }))

  expect((await sheet.findByRole('alert')).textContent).toBe(said)
  expect(router.state.location.pathname).toBe('/settings/account')
})

test('an address of letters a browser would refuse, such as č, deletes the account', async () => {
  const sent = server(undefined, 'čaba@primjer.hr')
  renderAt('/settings/account')

  fireEvent.click(await (await yourData()).findByRole('button', { name: 'Delete your account' }))
  const sheet = within(await screen.findByRole('dialog'))
  const field = sheet.getByLabelText('Your email address')
  expect(field.getAttribute('type')).toBe('text')
  expect(field.getAttribute('inputmode')).toBe('email')
  fireEvent.change(field, { target: { value: 'čaba@primjer.hr' } })
  fireEvent.click(sheet.getByRole('button', { name: 'Delete my account' }))

  expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeDefined()
  expect(sent).toEqual([{ email: 'čaba@primjer.hr' }])
})

test('an address typed in decomposed letters is the same address', async () => {
  const sent = server(undefined, 'čaba@primjer.hr')
  renderAt('/settings/account')

  fireEvent.click(await (await yourData()).findByRole('button', { name: 'Delete your account' }))
  const sheet = within(await screen.findByRole('dialog'))
  const decomposed = 'C\u030Caba@primjer.hr '
  fireEvent.change(sheet.getByLabelText('Your email address'), { target: { value: decomposed } })
  fireEvent.click(sheet.getByRole('button', { name: 'Delete my account' }))

  expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeDefined()
  expect(sent).toEqual([{ email: decomposed.trim() }])
})

test("a final Σ typed is the server's σ, as Go lowers it", async () => {
  const sent = server(undefined, 'οσ@example.com')
  renderAt('/settings/account')

  fireEvent.click(await (await yourData()).findByRole('button', { name: 'Delete your account' }))
  const sheet = within(await screen.findByRole('dialog'))
  fireEvent.change(sheet.getByLabelText('Your email address'), {
    target: { value: 'ΟΣ@example.com' },
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Delete my account' }))

  expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeDefined()
  expect(sent).toEqual([{ email: 'ΟΣ@example.com' }])
})
