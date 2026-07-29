#let ink = rgb("#17202a")
#let muted = rgb("#59636e")
#let accent = rgb("#006f7b")
#let rule = rgb("#c9d1d7")
#let draft-fill = rgb("#eef6f7")

#let paper(
  body,
  title: "",
  subtitle: "",
  author: "",
  revision: "",
) = {
  set document(
    title: title,
    author: author,
    keywords: (
      "mixed-phase FIR",
      "minimum phase",
      "group delay",
      "filter design",
      "reproducible research",
    ),
  )
  set page(
    paper: "a4",
    margin: (x: 24mm, top: 21mm, bottom: 23mm),
    numbering: "1",
    number-align: center,
  )
  set text(
    font: "Libertinus Serif",
    size: 10pt,
    fill: ink,
    lang: "en",
  )
  set par(justify: true, leading: 0.65em)
  set heading(numbering: "1.")
  set math.equation(numbering: "(1)")
  set table(
    stroke: rule,
    inset: (x: 5pt, y: 4pt),
  )
  show heading.where(level: 1): it => {
    v(0.8em)
    text(size: 13pt, weight: "bold", fill: accent, it)
  }
  show heading.where(level: 2): it => {
    v(0.45em)
    text(size: 11pt, weight: "bold", it)
  }
  show link: it => text(fill: accent, it)
  show raw.where(block: false): it => box(
    fill: rgb("#f1f3f5"),
    inset: (x: 2pt, y: 1pt),
    radius: 1.5pt,
    it,
  )

  align(center)[
    #text(size: 18pt, weight: "bold", title)
    #v(0.3em)
    #text(size: 11pt, fill: muted, subtitle)
    #v(1em)
    #text(weight: "bold", author)
    #linebreak()
    #text(size: 8.5pt, fill: muted)[Repository revision: #revision]
  ]

  v(0.8em)
  line(length: 100%, stroke: 0.8pt + accent)
  v(0.8em)

  body
}

#let abstract(body) = block(
  inset: (x: 10pt, y: 8pt),
  stroke: (left: 2pt + accent),
  fill: rgb("#f7f9fa"),
)[
  #strong[Abstract.] #body
]

#let draft-note(body) = block(
  width: 100%,
  inset: 8pt,
  radius: 2pt,
  fill: draft-fill,
  stroke: 0.5pt + accent,
)[
  #strong[Working-paper status.] #body
]

#let code-path(path) = text(size: 8.3pt, raw(path))
