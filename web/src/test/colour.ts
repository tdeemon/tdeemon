// The measures of a colour that the tests of the tokens and the charts'
// palette check them by: WCAG's contrast, OKLab's distance, and how a
// colour-blind eye sees one.

// resolved is a token's value in a theme, following the tokens it names:
// a colour, #rrggbb. A token the theme does not have, or one that is no
// such colour, throws, or a measure of it would be NaN, and NaN is less
// than no bound a test asks for.
export function resolved(theme: Record<string, string>, name: string): string {
  const value = theme[name]
  if (value === undefined) {
    throw new Error(`no token ${name}`)
  }
  const named = /^var\((--[\w-]+)\)$/.exec(value)?.[1]
  if (named !== undefined) {
    return resolved(theme, named)
  }
  if (!/^#[\da-f]{6}$/i.test(value)) {
    throw new Error(`${name} is ${value}, no #rrggbb`)
  }
  return value
}

type Rgb = [number, number, number]

// linear is a colour's channels, #rrggbb, as light, 0 to 1.
function linear(hex: string): Rgb {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = Number.parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.040_45 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return [r ?? 0, g ?? 0, b ?? 0]
}

// labOf is linear light in OKLab (Björn Ottosson's), where a distance is
// what an eye sees.
function labOf([r, g, b]: Rgb): Rgb {
  const l = Math.cbrt(0.412_221_470_8 * r + 0.536_332_536_3 * g + 0.051_445_992_9 * b)
  const m = Math.cbrt(0.211_903_498_2 * r + 0.680_699_545_1 * g + 0.107_396_956_6 * b)
  const s = Math.cbrt(0.088_302_461_9 * r + 0.281_718_837_6 * g + 0.629_978_700_5 * b)
  return [
    0.210_454_255_3 * l + 0.793_617_785 * m - 0.004_072_046_8 * s,
    1.977_998_495_1 * l - 2.428_592_205 * m + 0.450_593_709_9 * s,
    0.025_904_037_1 * l + 0.782_771_766_2 * m - 0.808_675_766 * s,
  ]
}

// oklab is a colour in OKLab.
function oklab(hex: string): Rgb {
  return labOf(linear(hex))
}

// lightness and chroma are a colour's L and C in OKLCH.
export function lightness(hex: string): number {
  return oklab(hex)[0]
}

export function chroma(hex: string): number {
  const [, a, b] = oklab(hex)
  return Math.hypot(a, b)
}

// distance is how far apart two colours are, in OKLab (0 to 1 from
// black to white).
export function distance(one: string, other: string): number {
  const [p, q] = [oklab(one), oklab(other)]
  return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2])
}

type Matrix = readonly [Rgb, Rgb, Rgb]

// Machado, Oliveira and Fernandes' (2009) simulation of a colour-blind
// eye at full severity, on linear light.
const blind: Record<'protan' | 'deutan', Matrix> = {
  protan: [
    [0.152_286, 1.052_583, -0.204_868],
    [0.114_503, 0.786_281, 0.099_216],
    [-0.003_882, -0.048_116, 1.051_998],
  ],
  deutan: [
    [0.367_322, 0.860_646, -0.227_968],
    [0.280_085, 0.672_501, 0.047_413],
    [-0.011_82, 0.042_94, 0.968_881],
  ],
}

// seen is a colour in OKLab as an eye whose matrix is one of blind's
// sees it.
function seen(hex: string, matrix: Matrix): Rgb {
  const c = linear(hex)
  const [r, g, b] = matrix.map((row) =>
    Math.min(1, Math.max(0, row[0] * c[0] + row[1] * c[1] + row[2] * c[2])),
  )
  return labOf([r ?? 0, g ?? 0, b ?? 0])
}

// blindDistance is how far apart two colours are to the eye of a
// protanope or a deuteranope, whichever tells them apart less.
export function blindDistance(one: string, other: string): number {
  return Math.min(
    ...Object.values(blind).map((matrix) => {
      const [p, q] = [seen(one, matrix), seen(other, matrix)]
      return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2])
    }),
  )
}

// luminance is a colour's relative luminance, as WCAG has it.
function luminance(hex: string): number {
  const [r, g, b] = linear(hex)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// contrast is WCAG's contrast ratio of two colours.
export function contrast(one: string, other: string): number {
  const [hi, lo] = [luminance(one), luminance(other)].toSorted((x, y) => y - x)
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05)
}
