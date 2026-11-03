import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { $api } from '@/api/client'
import type { components } from '@/api/schema'
import { setLocale } from '@/i18n/locale'
import { fakeServer, renderAt, signedIn, type Handler } from '@/test/app'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  setLocale('en')
  localStorage.clear()
})

type Household = components['schemas']['HouseholdBody']
type Invite = components['schemas']['HouseholdInviteBody']
type Report = components['schemas']['MergeReportBody']

const ana = { name: 'Ana', email: 'ana@example.com' }
const zoe = { name: 'Zoe', email: 'zoe@example.com' }
const mia = { name: 'Mia', email: 'mia@example.com' }

const fromMia: Invite = {
  id: '0193c3a5-0000-7000-8000-0000000000a1',
  by: mia,
  invited: ana,
  created_at: '2026-10-22T08:00:00Z',
  expires_at: '2026-10-29T08:00:00Z',
}
const ofZoe: Invite = {
  id: '0193c3a5-0000-7000-8000-0000000000a2',
  by: ana,
  invited: zoe,
  created_at: '2026-10-22T08:00:00Z',
  expires_at: '2026-10-29T08:00:00Z',
}

const report: Report = {
  merged: { foods: 3, recipes: 1 },
  received: { foods: 2, recipes: 0 },
  given: { foods: 0, recipes: 2 },
  renamed: [{ from: 'Porridge', to: 'Porridge (2)' }],
  dropped: [{ food: 'Kefir', code: '3850000000015' }],
}

// foodsAsked are the foods' queries a page of them would have asked: a
// join and a leave must have them asked again.
const foodsAsked = [
  $api.queryOptions('get', '/api/v1/foods/list').queryKey,
  $api.queryOptions('get', '/api/v1/foods', { params: { query: { query: ['oat'] } } }).queryKey,
]

// problemOf is a refusal of status whose error at where has code as its
// value, as the household's refusals say why.
function problemOf(status: number, where: string, code: string): Response {
  return Response.json(
    {
      status,
      title: 'Refused',
      detail: 'the invite is refused',
      errors: [{ location: where, value: code }],
    },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

// server is an instance whose signed-in user is Ana, with household as
// her household; it keeps it as the server does, and the bodies it was
// sent, by "METHOD /path".
function server(household: Household, over: Record<string, Handler> = {}) {
  const sent: Record<string, unknown[]> = {}
  let now = household
  const keep = async (key: string, r: Request) => {
    const text = await r.clone().text()
    ;(sent[key] ??= []).push(text === '' ? undefined : JSON.parse(text))
  }
  const routes: Record<string, Handler> = {
    ...signedIn(),
    'GET /api/v1/household': () => Response.json(now),
    'POST /api/v1/household/invites': async (r) => {
      await keep('POST /api/v1/household/invites', r)
      now = { ...now, sent: [ofZoe, ...now.sent] }
      return Response.json(ofZoe, { status: 201 })
    },
    'POST /api/v1/household/leave': async (r) => {
      await keep('POST /api/v1/household/leave', r)
      now = { ...now, members: [] }
      return new Response(null, { status: 204 })
    },
    [`GET /api/v1/household/invites/${fromMia.id}/preview`]: () =>
      Response.json({ report, fingerprint: 'f1' }),
    [`POST /api/v1/household/invites/${fromMia.id}/accept`]: async (r) => {
      await keep('accept', r)
      now = { members: [mia, { ...ana, you: true }], sent: [], received: [] }
      return Response.json(now)
    },
    ...Object.fromEntries(
      [fromMia, ofZoe].map((inv) => [
        `DELETE /api/v1/household/invites/${inv.id}`,
        async (r: Request) => {
          await keep(`DELETE ${inv.id}`, r)
          now = {
            ...now,
            sent: now.sent.filter((x) => x.id !== inv.id),
            received: now.received.filter((x) => x.id !== inv.id),
          }
          return new Response(null, { status: 204 })
        },
      ]),
    ),
    ...over,
  }
  const fetch = fakeServer(routes)
  return { fetch, sent }
}

const none: Household = { members: [], sent: [], received: [] }
const withZoe: Household = { members: [{ ...ana, you: true }, zoe], sent: [], received: [] }

test('the settings have a row to the household, which lists its members and the invites it sent', async () => {
  server({ ...withZoe, sent: [{ ...ofZoe, invited: mia }] })
  renderAt('/settings')

  const row = await screen.findByRole('link', { name: /Household/ })
  await waitFor(() => {
    expect(row.textContent).toContain('With Zoe')
  })
  fireEvent.click(row)

  expect(await screen.findByRole('heading', { name: 'Household', level: 1 })).toBeDefined()
  const members = within(await screen.findByRole('region', { name: 'Members' }))
  const rows = members.getAllByRole('listitem')
  expect(rows.map((r) => r.textContent)).toEqual(['Anaana@example.comYou', 'Zoezoe@example.com'])
  const sent = within(screen.getByRole('region', { name: 'Invites sent' }))
  expect(sent.getByText('mia@example.com')).toBeDefined()
  expect(sent.getByText(/^Waits until /)).toBeDefined()
})

test('a user in no household is told so, and invites someone in a sheet', async () => {
  const { sent } = server(none)
  renderAt('/settings/household')

  expect(await screen.findByText(/You are in no household/)).toBeDefined()
  expect(screen.queryByRole('region', { name: 'Members' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Invite someone' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'Invite someone' }))
  fireEvent.change(sheet.getByLabelText('Their email address'), {
    target: { value: ' zoe@example.com ' },
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Invite' }))

  expect(await screen.findByText('Invited zoe@example.com')).toBeDefined()
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'Invite someone' })).toBeNull()
  })
  expect(sent['POST /api/v1/household/invites']).toEqual([{ email: 'zoe@example.com' }])
  const list = within(await screen.findByRole('region', { name: 'Invites sent' }))
  expect(await list.findByText('Zoe')).toBeDefined()
})

test('an invite refused says why by its code, in Croatian too', async () => {
  server(none, {
    'POST /api/v1/household/invites': () => problemOf(409, 'body.email', 'other_household'),
  })
  renderAt('/settings/household')

  fireEvent.click(await screen.findByRole('button', { name: 'Invite someone' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'Invite someone' }))
  fireEvent.change(sheet.getByLabelText('Their email address'), {
    target: { value: 'mia@example.com' },
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Invite' }))
  expect((await sheet.findByRole('alert')).textContent).toBe(
    'They are in another household: they leave it first.',
  )
  cleanup()

  fakeServer({
    ...signedIn('hr'),
    'GET /api/v1/household': () => Response.json(none),
    'POST /api/v1/household/invites': () => problemOf(422, 'body.email', 'no_user'),
  })
  renderAt('/settings/household')
  fireEvent.click(await screen.findByRole('button', { name: 'Pozovi nekoga' }))
  const hr = within(await screen.findByRole('dialog', { name: 'Pozovi nekoga' }))
  fireEvent.change(hr.getByLabelText('E-mail adresa osobe'), {
    target: { value: 'nobody@example.com' },
  })
  fireEvent.click(hr.getByRole('button', { name: 'Pozovi' }))
  expect((await hr.findByRole('alert')).textContent).toBe(
    'Nijedan korisnik ovog servera nema tu adresu.',
  )
})

test('an invite for the user shows what joining does, and joins with its fingerprint', async () => {
  const { sent } = server({ ...none, received: [fromMia] })
  const queryClient = renderAt('/settings/household').options.context.queryClient
  for (const key of foodsAsked) {
    queryClient.setQueryData(key, {})
  }

  const received = within(await screen.findByRole('region', { name: 'Invites for you' }))
  expect(received.getByText('Mia invites you into their household')).toBeDefined()
  fireEvent.click(received.getByRole('button', { name: 'See what joining does' }))

  const sheet = within(await screen.findByRole('dialog', { name: 'What joining does' }))
  expect(await sheet.findByText('3 foods, 1 recipe')).toBeDefined()
  expect(sheet.getByText('2 foods')).toBeDefined()
  expect(sheet.getByText('2 recipes')).toBeDefined()
  expect(sheet.getByText('Porridge → Porridge (2)')).toBeDefined()
  expect(sheet.getByText('3850000000015 from Kefir')).toBeDefined()
  fireEvent.click(sheet.getByRole('button', { name: 'Join the household' }))

  expect(await screen.findByText('You joined Mia’s household')).toBeDefined()
  expect(sent.accept).toEqual([{ fingerprint: 'f1' }])
  // The kitchens merged: the foods are asked again.
  for (const key of foodsAsked) {
    expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true)
  }
  // The sheet's opener went with the invite: the focus stays on the page.
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Invite someone' }))
  })
  const members = within(await screen.findByRole('region', { name: 'Members' }))
  expect(members.getByText('Mia')).toBeDefined()
  expect(screen.queryByRole('region', { name: 'Invites for you' })).toBeNull()
})

test('an accept of a merge that changed shows the new one, which is joined', async () => {
  let previews = 0
  let accepts = 0
  const { sent } = server(
    { ...none, received: [fromMia] },
    {
      [`GET /api/v1/household/invites/${fromMia.id}/preview`]: () => {
        previews++
        return Response.json({
          report: previews === 1 ? report : { ...report, received: { foods: 5, recipes: 0 } },
          fingerprint: `f${String(previews)}`,
        })
      },
      [`POST /api/v1/household/invites/${fromMia.id}/accept`]: async (r) => {
        accepts++
        const body: unknown = await r.json()
        ;(sent.accept ??= []).push(body)
        return accepts === 1
          ? problemOf(409, 'body.fingerprint', 'merge_changed')
          : Response.json({ members: [mia, { ...ana, you: true }], sent: [], received: [] })
      },
    },
  )
  renderAt('/settings/household')

  fireEvent.click(await screen.findByRole('button', { name: 'See what joining does' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'What joining does' }))
  await sheet.findByText('2 foods')
  fireEvent.click(sheet.getByRole('button', { name: 'Join the household' }))

  expect(await sheet.findByText(/The foods changed since/)).toBeDefined()
  expect(await sheet.findByText('5 foods')).toBeDefined()
  await waitFor(() => {
    expect(sheet.getByRole('button', { name: 'Join the household' }).hasAttribute('disabled')).toBe(
      false,
    )
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Join the household' }))
  expect(await screen.findByText('You joined Mia’s household')).toBeDefined()
  expect(sent.accept).toEqual([{ fingerprint: 'f1' }, { fingerprint: 'f2' }])
})

test('an invite for the user is declined after it asks', async () => {
  const { sent } = server({ ...none, received: [fromMia] })
  renderAt('/settings/household')

  const received = within(await screen.findByRole('region', { name: 'Invites for you' }))
  fireEvent.click(received.getByRole('button', { name: 'Decline' }))
  expect(received.getByText('Decline Mia’s invite?')).toBeDefined()
  expect(document.activeElement?.textContent).toBe('Keep')
  fireEvent.click(received.getByRole('button', { name: 'Decline' }))

  expect(await screen.findByText('Declined Mia’s invite')).toBeDefined()
  expect(sent[`DELETE ${fromMia.id}`]).toHaveLength(1)
  await waitFor(() => {
    expect(screen.queryByRole('region', { name: 'Invites for you' })).toBeNull()
  })
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Invite someone' }))
  })
})

test('an invite the household sent is revoked after it asks; Keep keeps it', async () => {
  const { sent } = server({ ...withZoe, sent: [{ ...ofZoe, invited: mia }] })
  renderAt('/settings/household')

  const list = within(await screen.findByRole('region', { name: 'Invites sent' }))
  const revoke = list.getByRole('button', { name: 'Revoke the invite of Mia' })
  fireEvent.click(revoke)
  fireEvent.click(list.getByRole('button', { name: 'Keep' }))
  expect(document.activeElement).toBe(
    list.getByRole('button', { name: 'Revoke the invite of Mia' }),
  )
  expect(sent[`DELETE ${ofZoe.id}`]).toBeUndefined()

  fireEvent.click(list.getByRole('button', { name: 'Revoke the invite of Mia' }))
  fireEvent.click(list.getByRole('button', { name: 'Revoke' }))
  await waitFor(() => {
    expect(screen.queryByRole('region', { name: 'Invites sent' })).toBeNull()
  })
  expect(sent[`DELETE ${ofZoe.id}`]).toHaveLength(1)
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Invite someone' }))
  })
})

test('a member leaves after it asks what everyone keeps', async () => {
  const { sent } = server(withZoe)
  const queryClient = renderAt('/settings/household').options.context.queryClient
  for (const key of foodsAsked) {
    queryClient.setQueryData(key, {})
  }

  const members = within(await screen.findByRole('region', { name: 'Members' }))
  fireEvent.click(members.getByRole('button', { name: 'Leave the household' }))
  expect(members.getByText(/You keep every food and recipe you have, and so do they/)).toBeDefined()
  fireEvent.click(members.getByRole('button', { name: 'Leave' }))

  expect(await screen.findByText('You left the household')).toBeDefined()
  expect(sent['POST /api/v1/household/leave']).toHaveLength(1)
  for (const key of foodsAsked) {
    expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true)
  }
  expect(await screen.findByText(/You are in no household/)).toBeDefined()
  await waitFor(() => {
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Invite someone' }))
  })
})

test('a new preview that fails after the merge changed leaves nothing to join', async () => {
  let previews = 0
  server(
    { ...none, received: [fromMia] },
    {
      [`GET /api/v1/household/invites/${fromMia.id}/preview`]: () => {
        previews++
        return previews === 1
          ? Response.json({ report, fingerprint: 'f1' })
          : problemOf(404, 'path.id', 'gone')
      },
      [`POST /api/v1/household/invites/${fromMia.id}/accept`]: () =>
        problemOf(409, 'body.fingerprint', 'merge_changed'),
    },
  )
  renderAt('/settings/household')

  fireEvent.click(await screen.findByRole('button', { name: 'See what joining does' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'What joining does' }))
  await sheet.findByText('2 foods')
  fireEvent.click(sheet.getByRole('button', { name: 'Join the household' }))

  expect((await sheet.findByRole('alert')).textContent).toBe(
    'The invite is gone: it expired, or was revoked.',
  )
  expect(sheet.queryByText(/The foods changed since/)).toBeNull()
  expect(sheet.getByRole('button', { name: 'Join the household' }).hasAttribute('disabled')).toBe(
    true,
  )
})

test('an invite answered after its sheet was closed still says so', async () => {
  let answer: ((r: Response) => void) | undefined
  server(none, {
    'POST /api/v1/household/invites': () =>
      new Promise<Response>((resolve) => {
        answer = resolve
      }),
  })
  renderAt('/settings/household')

  fireEvent.click(await screen.findByRole('button', { name: 'Invite someone' }))
  const sheet = within(await screen.findByRole('dialog', { name: 'Invite someone' }))
  fireEvent.change(sheet.getByLabelText('Their email address'), {
    target: { value: 'zoe@example.com' },
  })
  fireEvent.click(sheet.getByRole('button', { name: 'Invite' }))
  await sheet.findByRole('button', { name: 'Inviting…' })
  fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
  await waitFor(() => {
    expect(screen.queryByRole('dialog', { name: 'Invite someone' })).toBeNull()
  })

  answer?.(Response.json(ofZoe, { status: 201 }))
  expect(await screen.findByText('Invited zoe@example.com')).toBeDefined()
})
