import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useLocation, useNavigate } from '@tanstack/react-router'
import { useEffect, useId, useState, type SubmitEvent } from 'react'

import { $api } from '@/api/client'
import { HttpError, NoAnswerError, statusOf } from '@/api/errors'
import { sessionQuery } from '@/api/session'
import { acceptInvite } from '@/api/sign-in'
import { AuthFrame, Or } from '@/components/auth-frame'
import { LoadError } from '@/components/load-error'
import { Skeleton } from '@/components/skeleton'
import { Button, buttonVariants } from '@/components/ui/button'
import type { Messages } from '@/i18n/en'
import { useFormat } from '@/i18n/format'
import { useLocale } from '@/i18n/locale'
import { useMessages } from '@/i18n/messages'
import { problem } from '@/i18n/problem'
import limits from '@/lib/limits.json'
import { fieldLabel, inputClass } from '@/lib/styles'
import { cn } from '@/lib/utils'

// browserZone is the time zone the new account starts in: the browser's.
function browserZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone
}

// passwordLength is a password's length as the server counts it: in
// characters, after NFKC.
function passwordLength(password: string): number {
  return Array.from(password.normalize('NFKC')).length
}

// refusal is what to tell a person whose account was not made.
function refusal(t: Messages, error: unknown): string {
  if (error instanceof HttpError) {
    switch (error.status) {
      case 404:
        return t.invite.broken
      case 409:
        return t.invite.taken
      case 422:
        return t.invite.refused
      case 503:
        return error.retryAfter === undefined ? t.serverFailed : t.signIn.busy(error.retryAfter)
      default:
        return t.invite.failed
    }
  }
  return error instanceof NoAnswerError ? t.unreachable : t.invite.failed
}

// InvitePage makes the account an invite's link is for: the link's token
// is after its #, which no request carries but the ones made here. It
// says the address the account will have, and makes it with a name and a
// password, or through the instance's OpenID Connect provider, in the
// browser's time zone and the app's language; then the account is
// signed in, on today's page. A link that does not work says so, with
// the way to sign in.
export function InvitePage() {
  const t = useMessages()
  const f = useFormat()
  const id = useId()
  const locale = useLocale()
  const token = useLocation({ select: (l) => l.hash })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const methods = $api.useQuery('get', '/api/v1/auth/methods')
  const check = $api.useQuery(
    'post',
    '/api/v1/invites/check',
    { body: { token } },
    { enabled: token !== '' },
  )
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  // What is wrong with a field, once the form was sent.
  const [wrong, setWrong] = useState<{ name?: string; password?: string }>({})
  const accept = useMutation({
    mutationFn: acceptInvite,
    onSuccess: async (session) => {
      // Nothing of another user stays in the app for the new one.
      queryClient.clear()
      queryClient.setQueryData(sessionQuery.queryKey, session)
      await navigate({ href: '/', replace: true })
    },
  })
  const sso = $api.useMutation('post', '/api/v1/invites/sso', {
    onSuccess: (answer) => {
      window.location.assign(answer.url)
    },
  })
  // Back from the provider's page through the browser's cache, the page
  // is as it was left: the provider's way is offered again.
  const resetSso = sso.reset
  useEffect(() => {
    const shown = (event: PageTransitionEvent) => {
      if (event.persisted) {
        resetSso()
      }
    }
    window.addEventListener('pageshow', shown)
    return () => {
      window.removeEventListener('pageshow', shown)
    }
  }, [resetSso])

  // One way to make the account at a time: while either is on its way,
  // and once one made it, neither is offered.
  const busy = accept.isPending || accept.isSuccess || sso.isPending || sso.isSuccess

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    if (busy) {
      return
    }
    const length = passwordLength(password)
    const found = {
      ...(name.trim() === '' ? { name: t.settings.nameNeeded } : {}),
      ...(length < limits.user.minPasswordLen
        ? { password: t.invite.tooShort(limits.user.minPasswordLen) }
        : length > limits.user.maxPasswordLen
          ? { password: t.invite.tooLong(limits.user.maxPasswordLen) }
          : {}),
    }
    setWrong(found)
    if (found.name !== undefined || found.password !== undefined) {
      return
    }
    accept.mutate({ token, name: name.trim(), password, timezone: browserZone(), locale })
  }

  const broken = token === '' || statusOf(check.error) === 404
  const oidc = methods.data?.oidc
  let body
  if (broken) {
    body = (
      <div className="space-y-4">
        <p role="alert">{t.invite.broken}</p>
        <Link to="/login" className={buttonVariants({ variant: 'outline' })}>
          {t.invite.toSignIn}
        </Link>
      </div>
    )
  } else if (check.isPending) {
    body = <Skeleton blocks={['h-6', 'h-44']} />
  } else if (check.data === undefined) {
    body = <LoadError error={check.error} onRetry={() => void check.refetch()} />
  } else {
    const invite = check.data
    body = (
      <>
        {/* Asked again and failed, the invite that came stays, and its form. */}
        {check.isError && (
          <output className="flex flex-wrap items-baseline gap-x-2 text-small text-muted-foreground">
            {problem(t, check.error)}
            <Button variant="link" className="h-auto p-0" onClick={() => void check.refetch()}>
              {t.retry}
            </Button>
          </output>
        )}
        <div className="space-y-1">
          <p>{t.invite.forEmail(invite.email)}</p>
          <p className="text-small text-muted-foreground">
            {t.invite.until(f.day(invite.expires_at, browserZone()))}
          </p>
        </div>
        <form onSubmit={submit} noValidate className="space-y-4">
          <div className="space-y-1">
            <label className="block space-y-1">
              <span className={cn(fieldLabel, 'block')}>{t.invite.name}</span>
              <input
                name="name"
                value={name}
                autoComplete="name"
                maxLength={limits.user.maxNameLen}
                required
                aria-invalid={wrong.name !== undefined || undefined}
                aria-describedby={wrong.name === undefined ? undefined : `${id}-name`}
                className={inputClass}
                onChange={(e) => {
                  setName(e.currentTarget.value)
                }}
              />
            </label>
            {wrong.name !== undefined && (
              <p id={`${id}-name`} role="alert" className="text-sm text-destructive">
                {wrong.name}
              </p>
            )}
          </div>
          <div className="space-y-1">
            <label className="block space-y-1">
              <span className={cn(fieldLabel, 'block')}>{t.invite.password}</span>
              <input
                name="password"
                type="password"
                value={password}
                autoComplete="new-password"
                required
                aria-invalid={wrong.password !== undefined || undefined}
                aria-describedby={`${id}-password`}
                className={inputClass}
                onChange={(e) => {
                  setPassword(e.currentTarget.value)
                }}
              />
            </label>
            {wrong.password === undefined ? (
              <p id={`${id}-password`} className="text-caption text-muted-foreground">
                {t.invite.passwordHint(limits.user.minPasswordLen)}
              </p>
            ) : (
              <p id={`${id}-password`} role="alert" className="text-sm text-destructive">
                {wrong.password}
              </p>
            )}
          </div>
          {accept.isError && (
            <p role="alert" className="text-destructive">
              {refusal(t, accept.error)}
            </p>
          )}
          <Button type="submit" className="w-full" disabled={busy}>
            {accept.isPending ? t.invite.submitting : t.invite.submit}
          </Button>
        </form>
        {oidc && (
          <>
            <Or />
            <Button
              variant="outline"
              className="w-full"
              disabled={busy}
              onClick={() => {
                sso.mutate({ body: { token, timezone: browserZone(), locale } })
              }}
            >
              {t.invite.with(oidc.name)}
            </Button>
            {sso.isError && (
              <p role="alert" className="text-destructive">
                {statusOf(sso.error) === 404 ? t.invite.broken : problem(t, sso.error)}
              </p>
            )}
          </>
        )}
      </>
    )
  }

  return <AuthFrame title={t.invite.title}>{body}</AuthFrame>
}
