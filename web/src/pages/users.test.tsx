import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import type { components } from '@/api/schema'
import { setLocale } from '@/i18n/locale'
import { fakeServer, problem, renderAt, session, signedIn, type Handler } from '@/test/app'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  setLocale('en')
  localStorage.clear()
})

type User = components['schemas']['AdminUserBody']
type Invite = components['schemas']['InviteBody']

const ana: User = {
  id: session().user.id,
  email: 'ana@example.com',
  name: 'Ana',
  role: 'admin',
  created_at: '2026-10-01T08:00:00Z',
}
const iva: User = {
  id: '0193c3a5-0000-7000-8000-000000000002',
  email: 'iva@example.com',
  name: 'Iva',
  role: 'user',
  created_at: '2026-10-02T08:00:00Z',
}
const marko: User = {
  id: '0193c3a5-0000-7000-8000-000000000003',
  email: 'marko@example.com',
  name: 'Marko',
  role: 'user',
  created_at: '2026-10-03T08:00:00Z',
  disabled_at: '2026-10-20T08:00:00Z',
}
// One waiting, one long expired, whatever day the tests run.
const waiting: Invite = {
  id: '0193c3a5-0000-7000-8000-0000000000e1',
  email: 'luka@example.com',
  role: 'user',
  created_at: '2026-10-22T08:00:00Z',
  expires_at: '2999-10-29T08:00:00Z',
}
const old: Invite = {
  id: '0193c3a5-0000-7000-8000-0000000000e2',
  email: 'petra@example.com',
  role: 'admin',
  created_at: '2026-01-01T08:00:00Z',
  expires_at: '2026-01-08T08:00:00Z',
}

// server is an instance whose signed-in user is Ana, an admin unless
// role says otherwise; it keeps the users and invites as the server
// does, and the bodies it was sent, by "METHOD /path".
function server(role: 'admin' | 'user' = 'admin', over: Record<string, Handler> = {}) {
  const sent: Record<string, unknown[]> = {}
  let users = [ana, iva, marko]
  let invites = [waiting, old]
  const keep = async (key: string, r: Request) => {
    const text = await r.clone().text()
    ;(sent[key] ??= []).push(text === '' ? undefined : JSON.parse(text))
  }
  const fetch = fakeServer({
    ...signedIn(),
    'GET /api/v1/auth/session': () =>
      Response.json({ ...session(), user: { ...session().user, role } }),
    'GET /api/v1/users': () => (role === 'admin' ? Response.json({ users }) : problem(403)),
    'GET /api/v1/invites': () => Response.json({ invites }),
    'POST /api/v1/invites': async (r) => {
      await keep('POST /api/v1/invites', r)
      const made: Invite = { ...waiting, id: 'new', email: 'nina@example.com', role: 'admin' }
      invites = [made, ...invites]
      return Response.json(
        { invite: made, url: 'http://localhost/invite#made-up-token' },
        { status: 201 },
      )
    },
    ...Object.fromEntries(
      [waiting, old].map((inv) => [
        `DELETE /api/v1/invites/${inv.id}`,
        async (r: Request) => {
          await keep(`DELETE /api/v1/invites/${inv.id}`, r)
          invites = invites.filter((x) => x.id !== inv.id)
          return new Response(null, { status: 204 })
        },
      ]),
    ),
    ...Object.fromEntries(
      [iva, marko].flatMap((u) => [
        [
          `PATCH /api/v1/users/${u.id}`,
          async (r: Request) => {
            await keep(`PATCH /api/v1/users/${u.id}`, r)
            const body: unknown = await r.json()
            const said = typeof body === 'object' && body !== null ? body : {}
            const active = 'active' in said ? said.active : undefined
            const made = 'role' in said && said.role === 'admin' ? 'admin' : 'user'
            users = users.map((x) => {
              if (x.id !== u.id) {
                return x
              }
              const { disabled_at: _, ...rest } = x
              const changed: User = { ...rest, ...('role' in said ? { role: made } : {}) }
              return active === false || (active === undefined && x.disabled_at !== undefined)
                ? { ...changed, disabled_at: '2026-10-23T08:00:00Z' }
                : changed
            })
            return Response.json(users.find((x) => x.id === u.id))
          },
        ],
        [
          `DELETE /api/v1/users/${u.id}`,
          async (r: Request) => {
            await keep(`DELETE /api/v1/users/${u.id}`, r)
            users = users.filter((x) => x.id !== u.id)
            return new Response(null, { status: 204 })
          },
        ],
      ]),
    ),
    ...over,
  })
  return { fetch, sent }
}

// element is an element found, which a test expects there.
function element(found: Element | null | undefined): HTMLElement {
  if (!(found instanceof HTMLElement)) {
    throw new Error('no such element')
  }
  return found
}

async function accounts() {
  return within(await screen.findByRole('region', { name: 'Accounts' }))
}

async function invitesRegion() {
  return within(await screen.findByRole('region', { name: 'Invites' }))
}

test('the admin has a row to the users, which lists the accounts and the invites', async () => {
  server()
  renderAt('/settings')

  fireEvent.click(await screen.findByRole('link', { name: 'Users' }))

  expect(await screen.findByRole('heading', { name: 'Users', level: 1 })).toBeDefined()
  const list = await accounts()
  const cards = await list.findAllByRole('listitem')
  expect(cards.map((c) => within(c).getByRole('heading').textContent)).toEqual([
    'Ana',
    'Iva',
    'Marko',
  ])
  // Her own account has no ways to change it here.
  expect(within(element(cards[0])).getByText('You')).toBeDefined()
  expect(within(element(cards[0])).queryByRole('button')).toBeNull()
  expect(within(element(cards[2])).getByText('Deactivated')).toBeDefined()
  expect(list.getByRole('button', { name: 'Activate: Marko' })).toBeDefined()
  expect(list.getByRole('button', { name: 'Deactivate: Iva' })).toBeDefined()
  expect(list.getByRole('button', { name: 'Make admin: Iva' })).toBeDefined()

  const invites = await invitesRegion()
  expect(await invites.findByText('luka@example.com')).toBeDefined()
  expect(invites.getByText(/^Works until /)).toBeDefined()
  const expired = invites.getByText('petra@example.com').closest('li')
  expect(within(element(expired)).getByText('Expired')).toBeDefined()
  expect(within(element(expired)).getByText('Admin')).toBeDefined()
})

test('a user who is no admin has no row to the users, and the page says it is the admin’s', async () => {
  const { fetch } = server('user')
  renderAt('/settings')

  expect(await screen.findByRole('link', { name: /Devices and AI coaches/ })).toBeDefined()
  expect(screen.queryByRole('link', { name: 'Users' })).toBeNull()
  cleanup()

  renderAt('/settings/users')
  expect(await screen.findByText('Only the admin manages users.')).toBeDefined()
  expect(screen.queryByRole('button', { name: 'Invite someone' })).toBeNull()
  expect(
    fetch.mock.calls.some(([r]) => r instanceof Request && r.url.endsWith('/api/v1/users')),
  ).toBe(false)
})

test('someone is invited in a sheet, which shows the link once, and the invite is listed', async () => {
  const { sent } = server()
  renderAt('/settings/users')

  const opener = await screen.findByRole('button', { name: 'Invite someone' })
  fireEvent.click(opener)
  const sheet = within(await screen.findByRole('dialog', { name: 'Invite someone' }))
  await waitFor(() => {
    expect(document.activeElement).toBe(sheet.getByLabelText('Email'))
  })
  fireEvent.change(sheet.getByLabelText('Email'), { target: { value: ' nina@example.com ' } })
  fireEvent.change(sheet.getByLabelText('Role'), { target: { value: 'admin' } })
  fireEvent.click(sheet.getByRole('button', { name: 'Make the invite link' }))

  expect(await sheet.findByText('http://localhost/invite#made-up-token')).toBeDefined()
  expect(sheet.getByText(/in the next 7 days/)).toBeDefined()
  expect(document.activeElement).toBe(sheet.getByRole('button', { name: 'Copy: The invite link' }))
  expect(sent['POST /api/v1/invites']).toEqual([{ email: 'nina@example.com', role: 'admin' }])

  fireEvent.click(sheet.getByRole('button', { name: 'Done' }))
  await waitFor(() => {
    expect(screen.queryByRole('dialog')).toBeNull()
  })
  expect(await (await invitesRegion()).findByText('nina@example.com')).toBeDefined()
})

test.each([
  [409, 'A user has this address already, or another admin invited it.'],
  [422, 'That is not an email address.'],
  [500, 'Not changed. The server failed to answer.'],
])('an invite the server refuses (%i) says why', async (status, said) => {
  server('admin', { 'POST /api/v1/invites': () => problem(status) })
  renderAt('/settings/users')

  fireEvent.click(await screen.findByRole('button', { name: 'Invite someone' }))
  const sheet = within(await screen.findByRole('dialog'))
  fireEvent.change(sheet.getByLabelText('Email'), { target: { value: 'ana@example.com' } })
  fireEvent.click(sheet.getByRole('button', { name: 'Make the invite link' }))

  expect((await sheet.findByRole('alert')).textContent).toBe(said)
  expect(sheet.getByLabelText('Email').getAttribute('aria-invalid')).toBe('true')
})

test('an invite is revoked after it asks, with the focus on Keep and back', async () => {
  const { sent } = server()
  renderAt('/settings/users')

  const invites = await invitesRegion()
  const revoke = await invites.findByRole('button', {
    name: 'Revoke the invite of luka@example.com',
  })
  fireEvent.click(revoke)
  expect(
    invites.getByText('Revoke the invite of luka@example.com? Its link stops working.'),
  ).toBeDefined()
  expect(document.activeElement).toBe(invites.getByRole('button', { name: 'Keep' }))
  fireEvent.click(invites.getByRole('button', { name: 'Keep' }))
  expect(document.activeElement).toBe(
    invites.getByRole('button', { name: 'Revoke the invite of luka@example.com' }),
  )

  fireEvent.click(invites.getByRole('button', { name: 'Revoke the invite of luka@example.com' }))
  fireEvent.click(invites.getByRole('button', { name: 'Revoke' }))
  await waitFor(() => {
    expect(invites.queryByText('luka@example.com')).toBeNull()
  })
  expect(sent[`DELETE /api/v1/invites/${waiting.id}`]).toHaveLength(1)
  expect(document.activeElement).toBe(invites.getByRole('heading', { name: 'Invites' }))
})

test('no invite waiting says so', async () => {
  server('admin', { 'GET /api/v1/invites': () => Response.json({ invites: [] }) })
  renderAt('/settings/users')

  expect(await (await invitesRegion()).findByText('No invite waits.')).toBeDefined()
})

test('a user is deactivated and made an admin after it asks, and activated at once', async () => {
  const { sent } = server()
  renderAt('/settings/users')

  const list = await accounts()
  // Deactivating asks, says what it does, and Keep sends nothing.
  fireEvent.click(await list.findByRole('button', { name: 'Deactivate: Iva' }))
  expect(
    list.getByText(
      'Deactivate Iva? They cannot sign in, their sessions end and their invites go. Their data stay.',
    ),
  ).toBeDefined()
  expect(document.activeElement).toBe(list.getByRole('button', { name: 'Keep' }))
  fireEvent.click(list.getByRole('button', { name: 'Keep' }))
  expect(document.activeElement).toBe(list.getByRole('button', { name: 'Deactivate: Iva' }))
  expect(sent[`PATCH /api/v1/users/${iva.id}`]).toBeUndefined()

  fireEvent.click(list.getByRole('button', { name: 'Deactivate: Iva' }))
  fireEvent.click(list.getByRole('button', { name: 'Deactivate' }))
  expect(await list.findByRole('button', { name: 'Activate: Iva' })).toBeDefined()

  // Activating takes nothing away: it asks nothing.
  fireEvent.click(list.getByRole('button', { name: 'Activate: Marko' }))
  expect(await list.findByRole('button', { name: 'Deactivate: Marko' })).toBeDefined()
  expect(list.queryByRole('button', { name: 'Keep' })).toBeNull()

  fireEvent.click(list.getByRole('button', { name: 'Make admin: Iva' }))
  expect(
    list.getByText('Make Iva an admin? They will manage this server and all its users.'),
  ).toBeDefined()
  fireEvent.click(list.getByRole('button', { name: 'Make admin' }))
  expect(await list.findByRole('button', { name: 'Make user: Iva' })).toBeDefined()
  expect(document.activeElement).toBe(list.getByRole('button', { name: 'Make user: Iva' }))

  fireEvent.click(list.getByRole('button', { name: 'Make user: Iva' }))
  expect(
    list.getByText('Make Iva a user? They stop managing this server, and their invites go.'),
  ).toBeDefined()
  fireEvent.click(list.getByRole('button', { name: 'Make user' }))
  expect(await list.findByRole('button', { name: 'Make admin: Iva' })).toBeDefined()

  expect(sent[`PATCH /api/v1/users/${iva.id}`]).toEqual([
    { active: false },
    { role: 'admin' },
    { role: 'user' },
  ])
  expect(sent[`PATCH /api/v1/users/${marko.id}`]).toEqual([{ active: true }])
})

test('a user is deleted after it asks, which says it is for good', async () => {
  const { sent } = server()
  renderAt('/settings/users')

  const list = await accounts()
  fireEvent.click(await list.findByRole('button', { name: 'Delete: Iva' }))
  expect(
    list.getByText('Delete Iva and all their data for good? It cannot be undone.'),
  ).toBeDefined()
  expect(document.activeElement).toBe(list.getByRole('button', { name: 'Keep' }))
  fireEvent.click(list.getByRole('button', { name: 'Keep' }))
  expect(document.activeElement).toBe(list.getByRole('button', { name: 'Delete: Iva' }))
  expect(sent[`DELETE /api/v1/users/${iva.id}`]).toBeUndefined()

  fireEvent.click(list.getByRole('button', { name: 'Delete: Iva' }))
  fireEvent.click(list.getByRole('button', { name: 'Delete for good' }))
  await waitFor(() => {
    expect(list.queryByText('Iva')).toBeNull()
  })
  expect(sent[`DELETE /api/v1/users/${iva.id}`]).toHaveLength(1)
  expect(document.activeElement).toBe(list.getByRole('heading', { name: 'Accounts' }))
})

test('a change that would leave no active admin says so on its card', async () => {
  const other: User = { ...iva, role: 'admin' }
  server('admin', {
    'GET /api/v1/users': () => Response.json({ users: [ana, other] }),
    [`PATCH /api/v1/users/${iva.id}`]: () => problem(409),
    [`DELETE /api/v1/users/${iva.id}`]: () => problem(500),
  })
  renderAt('/settings/users')

  const list = await accounts()
  fireEvent.click(await list.findByRole('button', { name: 'Make user: Iva' }))
  fireEvent.click(list.getByRole('button', { name: 'Make user' }))
  const card = element(list.getByText('Iva').closest('li'))
  expect((await within(card).findByRole('alert')).textContent).toBe(
    'This is the only active admin. Make another user an admin first.',
  )

  fireEvent.click(list.getByRole('button', { name: 'Keep' }))
  fireEvent.click(list.getByRole('button', { name: 'Delete: Iva' }))
  expect(within(card).queryByRole('alert')).toBeNull()
  fireEvent.click(list.getByRole('button', { name: 'Delete for good' }))
  expect((await within(card).findByRole('alert')).textContent).toBe(
    'Not changed. The server failed to answer.',
  )
})

test('users and invites that did not come say why, and come when asked again', async () => {
  let fails = true
  server('admin', {
    'GET /api/v1/users': () => (fails ? problem(500) : Response.json({ users: [ana] })),
    'GET /api/v1/invites': () => (fails ? problem(500) : Response.json({ invites: [] })),
  })
  renderAt('/settings/users')

  expect(await screen.findByText("Couldn't load the users.")).toBeDefined()
  expect(await screen.findByText("Couldn't load your invites.")).toBeDefined()
  fails = false
  for (const button of screen.getAllByRole('button', { name: 'Try again' })) {
    fireEvent.click(button)
  }
  expect(await (await accounts()).findByText('Ana')).toBeDefined()
  expect(await (await invitesRegion()).findByText('No invite waits.')).toBeDefined()
})

test('an address of letters a browser would refuse, such as č, is invited as typed', async () => {
  const { sent } = server()
  renderAt('/settings/users')

  fireEvent.click(await screen.findByRole('button', { name: 'Invite someone' }))
  const sheet = within(await screen.findByRole('dialog'))
  const field = sheet.getByLabelText('Email')
  expect(field.getAttribute('type')).toBe('text')
  expect(field.getAttribute('inputmode')).toBe('email')
  fireEvent.change(field, { target: { value: 'čaba@primjer.hr' } })
  fireEvent.click(sheet.getByRole('button', { name: 'Make the invite link' }))

  expect(await sheet.findByText('http://localhost/invite#made-up-token')).toBeDefined()
  expect(sent['POST /api/v1/invites']).toEqual([{ email: 'čaba@primjer.hr', role: 'user' }])
})
