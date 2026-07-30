#let chart-width = 720
#let chart-height = 465
#let plot-left = 118
#let plot-right = 700
#let plot-top = 24
#let plot-bottom = 315

#let ink = "#17202a"
#let muted = "#59636e"
#let grid-color = "#d9dee2"
#let method-colors = (
  "#007782",
  "#be6b00",
  "#4c5560",
  "#3f7a3f",
  "#944d73",
  "#1f4e79",
)

#let number(value, digits: 2) = str(calc.round(value, digits: digits))
#let x-linear(value, minimum, maximum) = (
  plot-left + (value - minimum) / (maximum - minimum) * (plot-right - plot-left)
)
#let y-linear(value, minimum, maximum) = (
  plot-bottom
    - (value - minimum) / (maximum - minimum) * (plot-bottom - plot-top)
)
#let log10(value) = calc.ln(value) / calc.ln(10)
#let y-log(value, minimum, maximum) = y-linear(
  log10(value),
  log10(minimum),
  log10(maximum),
)
#let x-log(value, minimum, maximum) = (
  plot-left
    + (log10(value) - log10(minimum))
      / (log10(maximum) - log10(minimum))
      * (plot-right - plot-left)
)

#let method-index(method) = if method == "budde-iterative" {
  0
} else if method == "phase-interpolation" {
  1
} else if method == "complex-minimax" {
  2
} else if method == "minphase-truncation" {
  3
} else if method == "low-group-delay" {
  4
} else {
  5
}

#let method-label(method) = if method == "budde-iterative" {
  "Alternating"
} else if method == "phase-interpolation" {
  "Phase interpolation"
} else if method == "complex-minimax" {
  "Complex minimax"
} else if method == "minphase-truncation" {
  "Minimum-phase truncation"
} else if method == "low-group-delay" {
  "Low group delay"
} else {
  "Alternating, selected delay"
}

#let reference-targets = (
  "low-pass",
  "parametric-eq",
  "crossover",
  "deep-notch",
  "room-correction",
  "steep-crossover",
)

#let cross-target-win-count(rows, method, key) = {
  let wins = reference-targets.filter(target => {
    let target-rows = rows.filter(row => row.at("target") == target)
    let method-row = target-rows.find(row => row.at("method") == method)
    let value = float(method-row.at(key))
    target-rows.all(row => value <= float(row.at(key)))
  })
  wins.len()
}

#let method-dash(method) = if method == "budde-iterative" {
  none
} else if method == "phase-interpolation" {
  "13 7"
} else if method == "complex-minimax" {
  "3 6"
} else if method == "minphase-truncation" {
  "2 4"
} else if method == "low-group-delay" {
  "13 5 3 5"
} else {
  "7 4 2 4"
}

#let svg-text(
  x,
  y,
  label,
  anchor: "middle",
  size: 18,
  fill: ink,
  weight: "normal",
  transform: none,
) = {
  let transform-attr = if transform == none {
    ""
  } else {
    (" transform=\"", transform, "\"").map(str).join("")
  }
  (
    "<text x=\"",
    number(x),
    "\" y=\"",
    number(y),
    "\" text-anchor=\"",
    anchor,
    "\" font-family=\"DejaVu Sans, sans-serif\" font-size=\"",
    str(size),
    "\" font-weight=\"",
    weight,
    "\" fill=\"",
    fill,
    "\"",
    transform-attr,
    ">",
    label,
    "</text>",
  )
    .map(str)
    .join("")
}

#let svg-line(
  x1,
  y1,
  x2,
  y2,
  stroke: ink,
  width: 2,
  dash: none,
) = {
  let dash-attr = if dash == none {
    ""
  } else {
    (" stroke-dasharray=\"", dash, "\"").map(str).join("")
  }
  (
    "<line x1=\"",
    number(x1),
    "\" y1=\"",
    number(y1),
    "\" x2=\"",
    number(x2),
    "\" y2=\"",
    number(y2),
    "\" stroke=\"",
    stroke,
    "\" stroke-width=\"",
    str(width),
    "\"",
    dash-attr,
    "/>",
  )
    .map(str)
    .join("")
}

#let marker(method, x, y, size: 8) = {
  let index = method-index(method)
  let color = method-colors.at(index)
  if index == 0 {
    (
      "<circle cx=\"",
      number(x),
      "\" cy=\"",
      number(y),
      "\" r=\"",
      str(size),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  } else if index == 1 {
    (
      "<rect x=\"",
      number(x - size),
      "\" y=\"",
      number(y - size),
      "\" width=\"",
      str(2 * size),
      "\" height=\"",
      str(2 * size),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  } else if index == 2 {
    (
      "<polygon points=\"",
      number(x),
      ",",
      number(y - size - 1),
      " ",
      number(x - size),
      ",",
      number(y + size),
      " ",
      number(x + size),
      ",",
      number(y + size),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  } else if index == 3 {
    // Inverted triangle. Every method needs a shape of its own: the paper
    // claims the figures stay legible in greyscale, which only holds while no
    // two methods share a marker.
    (
      "<polygon points=\"",
      number(x),
      ",",
      number(y + size + 1),
      " ",
      number(x - size),
      ",",
      number(y - size),
      " ",
      number(x + size),
      ",",
      number(y - size),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  } else if index == 4 {
    (
      "<polygon points=\"",
      number(x),
      ",",
      number(y - size - 1),
      " ",
      number(x - size - 1),
      ",",
      number(y),
      " ",
      number(x),
      ",",
      number(y + size + 1),
      " ",
      number(x + size + 1),
      ",",
      number(y),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  } else {
    // Cross. The sixth method needs a sixth outline, because the paper's claim
    // that the figures survive greyscale printing holds only while every method
    // is identifiable by shape alone.
    let arm = size / 3
    (
      "<polygon points=\"",
      number(x - arm),
      ",",
      number(y - size),
      " ",
      number(x + arm),
      ",",
      number(y - size),
      " ",
      number(x + arm),
      ",",
      number(y - arm),
      " ",
      number(x + size),
      ",",
      number(y - arm),
      " ",
      number(x + size),
      ",",
      number(y + arm),
      " ",
      number(x + arm),
      ",",
      number(y + arm),
      " ",
      number(x + arm),
      ",",
      number(y + size),
      " ",
      number(x - arm),
      ",",
      number(y + size),
      " ",
      number(x - arm),
      ",",
      number(y + arm),
      " ",
      number(x - size),
      ",",
      number(y + arm),
      " ",
      number(x - size),
      ",",
      number(y - arm),
      " ",
      number(x - arm),
      ",",
      number(y - arm),
      "\" fill=\"",
      color,
      "\" stroke=\"",
      ink,
      "\" stroke-width=\"1.5\"/>",
    )
      .map(str)
      .join("")
  }
}

// xy-polyline draws a polyline from two named columns. The rows are used in the
// order given, so a caller whose x column is not already sorted must sort first.
#let xy-polyline(
  rows,
  x-position,
  y-position,
  x-key,
  y-key,
  stroke,
  width: 3,
  dash: none,
) = {
  if rows.len() == 0 {
    ""
  } else {
    let points = rows
      .map(row => (
        number(x-position(float(row.at(x-key)))),
        ",",
        number(y-position(float(row.at(y-key)))),
      ).join(""))
      .join(" ")
    let dash-attr = if dash == none {
      ""
    } else {
      (" stroke-dasharray=\"", dash, "\"").map(str).join("")
    }
    (
      "<polyline points=\"",
      points,
      "\" fill=\"none\" stroke=\"",
      stroke,
      "\" stroke-width=\"",
      str(width),
      "\" stroke-linejoin=\"round\" stroke-linecap=\"round\"",
      dash-attr,
      "/>",
    )
      .map(str)
      .join("")
  }
}

#let polyline(
  rows,
  x-position,
  y-position,
  value-key,
  stroke,
  width: 3,
  dash: none,
) = {
  if rows.len() == 0 {
    ""
  } else {
    let points = rows
      .map(row => (
        number(x-position(float(row.at("frequency_hz")))),
        ",",
        number(y-position(float(row.at(value-key)))),
      ).join(""))
      .join(" ")
    let dash-attr = if dash == none {
      ""
    } else {
      (" stroke-dasharray=\"", dash, "\"").map(str).join("")
    }
    (
      "<polyline points=\"",
      points,
      "\" fill=\"none\" stroke=\"",
      stroke,
      "\" stroke-width=\"",
      str(width),
      "\" stroke-linejoin=\"round\" stroke-linecap=\"round\"",
      dash-attr,
      "/>",
    )
      .map(str)
      .join("")
  }
}

#let impulse-polyline(
  rows,
  x-position,
  y-position,
  stroke,
  width: 3,
  dash: none,
) = {
  if rows.len() == 0 {
    ""
  } else {
    let amplitude-floor = 0.0001
    let points = rows
      .map(row => {
        let amplitude = calc.abs(float(row.at("normalised_coefficient")))
        let level = if amplitude <= amplitude-floor {
          -80
        } else {
          20 * log10(amplitude)
        }
        (
          number(x-position(float(row.at("peak_aligned_index")))),
          ",",
          number(y-position(level)),
        ).join("")
      })
      .join(" ")
    let dash-attr = if dash == none {
      ""
    } else {
      (" stroke-dasharray=\"", dash, "\"").map(str).join("")
    }
    (
      "<polyline points=\"",
      points,
      "\" fill=\"none\" stroke=\"",
      stroke,
      "\" stroke-width=\"",
      str(width),
      "\" stroke-linejoin=\"round\" stroke-linecap=\"round\"",
      dash-attr,
      "/>",
    )
      .map(str)
      .join("")
  }
}

#let svg-image(width, height, body, definitions: "") = {
  let svg = (
    "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 ",
    str(width),
    " ",
    str(height),
    "\"><rect width=\"100%\" height=\"100%\" fill=\"white\"/><defs>",
    definitions,
    "</defs>",
    body,
    "</svg>",
  )
    .map(str)
    .join("")
  image(bytes(svg), format: "svg", width: 100%)
}

// height overrides the canvas for charts whose legend needs more rows than the
// default leaves room for. The plot area itself is fixed by plot-top and
// plot-bottom, so a taller canvas only adds space below the axis label.
#let chart-image(body, definitions: "", height: chart-height) = {
  svg-image(
    chart-width,
    height,
    body,
    definitions: definitions,
  )
}

#let signal-flow-block(x, y, width, title, detail, fill: "#f1f3f5") = (
  "<rect x=\"",
  number(x),
  "\" y=\"",
  number(y),
  "\" width=\"",
  number(width),
  "\" height=\"58\" rx=\"3\" fill=\"",
  fill,
  "\" stroke=\"",
  ink,
  "\" stroke-width=\"2\"/>",
  svg-text(x + width / 2, y + 25, title, size: 16, weight: "bold"),
  svg-text(x + width / 2, y + 45, detail, size: 15, fill: muted),
)
.map(str)
.join("")

#let signal-flow-arrow(x1, y, x2) = (
  svg-line(x1, y, x2, y, width: 2),
  "<polygon points=\"",
  number(x2),
  ",",
  number(y),
  " ",
  number(x2 - 9),
  ",",
  number(y - 5),
  " ",
  number(x2 - 9),
  ",",
  number(y + 5),
  "\" fill=\"",
  ink,
  "\"/>",
)
.map(str)
.join("")

#let signal-flow-diagram() = {
  let body = (
    svg-text(
      18,
      24,
      "Earlier hybrid equaliser (a)",
      anchor: "start",
      size: 17,
      fill: muted,
      weight: "bold",
    ),
    svg-text(52, 77, "Input", size: 16),
    signal-flow-arrow(79, 72, 126),
    signal-flow-block(
      126,
      43,
      190,
      "Minimum-phase IIR",
      "filter bank",
    ),
    signal-flow-arrow(316, 72, 365),
    signal-flow-block(
      365,
      43,
      190,
      "Linear-phase FIR",
      "correction filter",
    ),
    signal-flow-arrow(555, 72, 610),
    svg-text(660, 67, "Mixed-phase", size: 16, weight: "bold"),
    svg-text(660, 88, "output", size: 15, fill: muted),
    svg-line(18, 124, 702, 124, stroke: grid-color, width: 1),
    svg-text(
      18,
      146,
      "2012 all-FIR factorisation (b)",
      anchor: "start",
      size: 17,
      fill: muted,
      weight: "bold",
    ),
    svg-text(52, 193, "Input", size: 16),
    signal-flow-arrow(79, 188, 126),
    signal-flow-block(
      126,
      159,
      190,
      "Minimum-phase FIR",
      "a[n], N_A taps",
      fill: "#e7f3f4",
    ),
    signal-flow-arrow(316, 188, 365),
    signal-flow-block(
      365,
      159,
      190,
      "Linear-phase FIR",
      "b[n], N_B = 2d + 1 taps",
      fill: "#f8ecdc",
    ),
    signal-flow-arrow(555, 188, 610),
    svg-text(660, 183, "h[n] = a * b", size: 16, weight: "bold"),
    svg-text(660, 204, "N taps", size: 15, fill: muted),
    svg-text(
      340,
      244,
      "Convolution support: N_A + N_B - 1 = N",
      size: 16,
      fill: muted,
    ),
  )
    .flatten()
    .map(str)
    .join("")
  svg-image(720, 260, body)
}

#let axes(
  x-ticks,
  y-ticks,
  x-position,
  y-position,
  x-label,
  y-label,
) = {
  // An empty tick list is a legitimate request: the lane charts label their rows
  // inside the plot and want no vertical scale at all. It has to be special-cased
  // because joining an empty array yields none rather than an empty string.
  let horizontal = if y-ticks.len() == 0 {
    ""
  } else {
    y-ticks
      .map(tick => {
        let y = y-position(tick.at(0))
        (
          svg-line(plot-left, y, plot-right, y, stroke: grid-color, width: 1),
          svg-text(plot-left - 12, y + 6, tick.at(1), anchor: "end", size: 17),
        )
          .map(str)
          .join("")
      })
      .join("")
  }
  let vertical = x-ticks
    .map(tick => {
      let x = x-position(tick.at(0))
      (
        svg-line(x, plot-top, x, plot-bottom, stroke: grid-color, width: 1),
        svg-text(x, plot-bottom + 24, tick.at(1), size: 17),
      )
        .map(str)
        .join("")
    })
    .join("")
  (
    horizontal,
    vertical,
    svg-line(plot-left, plot-bottom, plot-right, plot-bottom, width: 2),
    svg-line(plot-left, plot-top, plot-left, plot-bottom, width: 2),
    svg-text(
      (plot-left + plot-right) / 2,
      plot-bottom + 59,
      x-label,
      size: 19,
      weight: "bold",
    ),
    svg-text(
      24,
      (plot-top + plot-bottom) / 2,
      y-label,
      size: 18,
      weight: "bold",
      transform: (
        "rotate(-90 24 ",
        number((plot-top + plot-bottom) / 2),
        ")",
      )
        .map(str)
        .join(""),
    ),
  )
    .map(str)
    .join("")
}

// Six methods fill three rows of two exactly. The block starts high enough to
// keep the last row's descenders inside the canvas.
#let method-legend(y: 396) = {
  let methods = (
    "budde-iterative",
    "budde-adaptive",
    "phase-interpolation",
    "complex-minimax",
    "minphase-truncation",
    "low-group-delay",
  )
  methods
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let method = pair.at(1)
      let x = plot-left + calc.rem(index, 2) * 290
      let row-y = y + calc.floor(index / 2) * 28
      (
        marker(method, x, row-y - 5, size: 6),
        svg-text(
          x + 13,
          row-y,
          method-label(method),
          anchor: "start",
          size: 17,
        ),
      )
        .map(str)
        .join("")
    })
    .join("")
}

// Four rows of two now, since six methods plus an optional target entry no
// longer fit in three. The callers pass a taller canvas to match.
#let response-legend(include-target: false, y: 401) = {
  let entries = if include-target {
    (
      ("target", "Target"),
      ("budde-iterative", method-label("budde-iterative")),
      ("budde-adaptive", method-label("budde-adaptive")),
      ("phase-interpolation", method-label("phase-interpolation")),
      ("complex-minimax", method-label("complex-minimax")),
      ("minphase-truncation", method-label("minphase-truncation")),
      ("low-group-delay", method-label("low-group-delay")),
    )
  } else {
    (
      ("budde-iterative", method-label("budde-iterative")),
      ("budde-adaptive", method-label("budde-adaptive")),
      ("phase-interpolation", method-label("phase-interpolation")),
      ("complex-minimax", method-label("complex-minimax")),
      ("minphase-truncation", method-label("minphase-truncation")),
      ("low-group-delay", method-label("low-group-delay")),
    )
  }
  entries
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let method = pair.at(1).at(0)
      let label = pair.at(1).at(1)
      let x = plot-left + calc.rem(index, 2) * 292
      let row-y = y + calc.floor(index / 2) * 27
      let color = if method == "target" {
        ink
      } else {
        method-colors.at(method-index(method))
      }
      let dash = if method == "target" {
        "8 5"
      } else {
        method-dash(method)
      }
      (
        svg-line(
          x,
          row-y - 5,
          x + 34,
          row-y - 5,
          stroke: color,
          width: 4,
          dash: dash,
        ),
        svg-text(x + 44, row-y, label, anchor: "start", size: 16),
      )
        .map(str)
        .join("")
    })
    .join("")
}

#let accuracy-delay-chart(rows) = {
  // The axis spans every target, not just the smooth ones: the steep-crossover
  // designs sit near 50 samples, so a 30-sample axis would drop them outside
  // the plot frame entirely.
  let x-pos(value) = x-linear(value, 0, 60)
  // Ten decades, and clamped. The minimum-phase designs on the two crossovers
  // reach 1.6e-8%, three decades below the old floor, and this chart draws no
  // clip path: an unclamped point does not vanish, it renders below the frame.
  let y-min = 0.00000001
  let y-max = 100
  let y-pos(value) = y-log(calc.clamp(value, y-min, y-max), y-min, y-max)
  let points = rows
    .map(row => {
      let delay = float(row.at("mean_group_delay"))
      let error = float(row.at("relative_magnitude_error")) * 100
      marker(row.at("method"), x-pos(delay), y-pos(error))
    })
    .join("")
  let chart-axes = axes(
    ((0, "0"), (15, "15"), (30, "30"), (45, "45"), (60, "60")),
    (
      (0.00000001, "1e-8"),
      (0.000001, "1e-6"),
      (0.0001, "1e-4"),
      (0.01, "0.01"),
      (1, "1"),
      (100, "100"),
    ),
    x-pos,
    y-pos,
    "Mean group delay (samples)",
    "Relative magnitude error (%)",
  )
  chart-image((chart-axes, points, method-legend()).map(str).join(""))
}

#let pre-ringing-chart(rows) = {
  let targets = (
    ("low-pass", "LP"),
    ("parametric-eq", "PEQ"),
    ("crossover", "XO"),
    ("deep-notch", "Notch"),
    ("room-correction", "Room"),
    ("steep-crossover", "Steep XO"),
  )
  let methods = (
    "budde-iterative",
    "budde-adaptive",
    "phase-interpolation",
    "complex-minimax",
    "minphase-truncation",
    "low-group-delay",
  )
  // Headroom above the 49.45% maximum so no bar renders flush against the
  // top rule.
  let y-pos(value) = y-linear(value, 0, 55)
  let group-width = (plot-right - plot-left) / 6
  // Six bars per group have to fit inside the 97-point group width: at the old
  // 14-point bar the sixth would have started past the group boundary and
  // overlapped the next target.
  let bar-width = 11
  let bar-gap = 2
  let group-inset = 10
  let bars = targets
    .enumerate()
    .map(target-pair => {
      let target-index = target-pair.at(0)
      let target = target-pair.at(1).at(0)
      methods
        .enumerate()
        .map(method-pair => {
          let slot = method-pair.at(0)
          let method = method-pair.at(1)
          let row = rows.find(row => (
            row.at("target") == target and row.at("method") == method
          ))
          let value = float(row.at("pre_peak_energy_ratio")) * 100
          let x = (
            plot-left
              + target-index * group-width
              + group-inset
              + slot * (bar-width + bar-gap)
          )
          let y = y-pos(value)
          (
            "<rect x=\"",
            number(x),
            "\" y=\"",
            number(y),
            "\" width=\"",
            str(bar-width),
            "\" height=\"",
            number(plot-bottom - y),
            "\" fill=\"url(#pattern",
            str(method-index(method)),
            ")\" stroke=\"",
            ink,
            "\" stroke-width=\"1\"/>",
          )
            .map(str)
            .join("")
        })
        .join("")
    })
    .join("")
  let x-ticks = targets
    .enumerate()
    .map(pair => (
      pair.at(0),
      pair.at(1).at(1),
    ))
  let x-pos(value) = plot-left + (value + 0.5) * group-width
  let chart-axes = axes(
    x-ticks,
    ((0, "0"), (10, "10"), (20, "20"), (30, "30"), (40, "40"), (50, "50")),
    x-pos,
    y-pos,
    "Target",
    "Energy before peak (%)",
  )
  // One pattern per method, in method-index order, each matching that method's
  // line colour. A missing entry would silently render an unfilled bar.
  let definitions = "
    <pattern id=\"pattern0\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#007782\"/>
    </pattern>
    <pattern id=\"pattern1\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#f2d2a9\"/>
      <path d=\"M-2,2 L2,-2 M0,8 L8,0 M6,10 L10,6\" stroke=\"#7a4b00\" stroke-width=\"2\"/>
    </pattern>
    <pattern id=\"pattern2\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#d9dde0\"/>
      <circle cx=\"2\" cy=\"2\" r=\"1.4\" fill=\"#33383d\"/>
      <circle cx=\"6\" cy=\"6\" r=\"1.4\" fill=\"#33383d\"/>
    </pattern>
    <pattern id=\"pattern3\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#d6e6d6\"/>
      <path d=\"M0,4 L8,4\" stroke=\"#2c562c\" stroke-width=\"1.6\"/>
    </pattern>
    <pattern id=\"pattern4\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#ead5e1\"/>
      <path d=\"M0,0 L8,8 M8,0 L0,8\" stroke=\"#65334f\" stroke-width=\"1.4\"/>
    </pattern>
    <pattern id=\"pattern5\" width=\"8\" height=\"8\" patternUnits=\"userSpaceOnUse\">
      <rect width=\"8\" height=\"8\" fill=\"#cfdcea\"/>
      <path d=\"M4,0 L4,8\" stroke=\"#1f4e79\" stroke-width=\"1.6\"/>
    </pattern>"
  chart-image(
    (chart-axes, bars, method-legend()).map(str).join(""),
    definitions: definitions,
  )
}

#let graphiceq-chart(rows) = {
  let hybrid = rows.filter(row => row.at("method") == "hybrid")
  let fir = rows.filter(row => row.at("method") == "all-fir-equal-latency")
  let x-pos(value) = x-linear(value, 0, 1600)
  let y-pos(value) = y-linear(value, 0, 0.35)
  let chart-axes = axes(
    ((0, "0"), (400, "400"), (800, "800"), (1200, "1200"), (1600, "1600")),
    ((0, "0"), (0.1, "0.1"), (0.2, "0.2"), (0.3, "0.3")),
    x-pos,
    y-pos,
    "Latency (samples)",
    "RMS magnitude error (dB)",
  )
  let hybrid-points = hybrid
    .map(row => {
      let x = x-pos(float(row.at("latency")))
      let y = y-pos(float(row.at("rms_error_db")))
      (
        "<circle cx=\"",
        number(x),
        "\" cy=\"",
        number(y),
        "\" r=\"8\" fill=\"#007782\" stroke=\"",
        ink,
        "\" stroke-width=\"1.5\"/>",
      )
        .map(str)
        .join("")
    })
    .join("")
  let fir-points = fir
    .map(row => {
      let x = x-pos(float(row.at("latency")))
      let y = y-pos(float(row.at("rms_error_db")))
      (
        "<rect x=\"",
        number(x - 8),
        "\" y=\"",
        number(y - 8),
        "\" width=\"16\" height=\"16\" fill=\"#f2d2a9\" stroke=\"",
        ink,
        "\" stroke-width=\"1.5\"/>",
      )
        .map(str)
        .join("")
    })
    .join("")
  let legend = (
    "<circle cx=\"150\" cy=\"414\" r=\"7\" fill=\"#007782\" stroke=\"",
    ink,
    "\"/><text x=\"166\" y=\"420\" font-family=\"DejaVu Sans, sans-serif\" font-size=\"18\" fill=\"",
    ink,
    "\">Hybrid IIR/FIR</text><rect x=\"405\" y=\"407\" width=\"14\" height=\"14\" fill=\"#f2d2a9\" stroke=\"",
    ink,
    "\"/><text x=\"428\" y=\"420\" font-family=\"DejaVu Sans, sans-serif\" font-size=\"18\" fill=\"",
    ink,
    "\">All-FIR, equal latency</text>",
  )
    .map(str)
    .join("")
  chart-image((chart-axes, hybrid-points, fir-points, legend).map(str).join(""))
}

// The response CSV carries more than one target, so every chart selects its
// own. Axis bounds are per-target: an equaliser's ±12 dB window says nothing
// useful about a crossover that falls past -100 dB.
#let magnitude-response-chart(
  rows,
  target: "parametric-eq",
  y-bounds: (-4, 12),
  y-ticks: ((-4, "-4"), (0, "0"), (4, "4"), (8, "8"), (12, "12")),
) = {
  let visible = rows.filter(row => {
    let frequency = float(row.at("frequency_hz"))
    row.at("target") == target and frequency >= 50 and frequency <= 20000
  })
  let y-min = y-bounds.at(0)
  let y-max = y-bounds.at(1)
  let x-pos(value) = x-log(value, 50, 20000)
  // Clamped so a null deeper than the axis pins to the frame instead of
  // escaping it. The floor is stated in the caption.
  let y-pos(value) = y-linear(calc.clamp(value, y-min, y-max), y-min, y-max)
  let chart-axes = axes(
    ((50, "50"), (100, "100"), (1000, "1k"), (10000, "10k"), (20000, "20k")),
    y-ticks,
    x-pos,
    y-pos,
    "Frequency (Hz)",
    "Magnitude (dB)",
  )
  let target = visible.filter(row => row.at("method") == "budde-iterative")
  let target-line = polyline(
    target,
    x-pos,
    y-pos,
    "target_magnitude_db",
    ink,
    width: 4,
    dash: "8 5",
  )
  let methods = (
    "budde-iterative",
    "budde-adaptive",
    "phase-interpolation",
    "complex-minimax",
    "minphase-truncation",
    "low-group-delay",
  )
  let lines = methods
    .map(method => polyline(
      visible.filter(row => row.at("method") == method),
      x-pos,
      y-pos,
      "magnitude_db",
      method-colors.at(method-index(method)),
      dash: method-dash(method),
    ))
    .join("")
  chart-image(
    height: chart-height + 30,
    (chart-axes, lines, target-line, response-legend(include-target: true))
      .map(str)
      .join(""),
  )
}

#let group-delay-response-chart(
  rows,
  target: "parametric-eq",
  x-bounds: (1800, 5000),
  x-ticks: (
    (1800, "1.8k"),
    (2600, "2.6k"),
    (3400, "3.4k"),
    (4200, "4.2k"),
    (5000, "5k"),
  ),
  y-bounds: (-40, 40),
  y-ticks: ((-40, "-40"), (-20, "-20"), (0, "0"), (20, "20"), (40, "40")),
) = {
  let visible = rows.filter(row => (
    row.at("target") == target and float(row.at("delay_weight")) > 0
  ))
  let y-min = y-bounds.at(0)
  let y-max = y-bounds.at(1)
  let x-pos(value) = x-linear(value, x-bounds.at(0), x-bounds.at(1))
  let y-pos(value) = y-linear(calc.clamp(value, y-min, y-max), y-min, y-max)
  let chart-axes = axes(
    x-ticks,
    y-ticks,
    x-pos,
    y-pos,
    "Frequency (Hz)",
    "Group delay (samples)",
  )
  let methods = (
    "budde-iterative",
    "budde-adaptive",
    "phase-interpolation",
    "complex-minimax",
    "minphase-truncation",
    "low-group-delay",
  )
  let lines = methods
    .map(method => polyline(
      visible.filter(row => row.at("method") == method),
      x-pos,
      y-pos,
      "group_delay_samples",
      method-colors.at(method-index(method)),
      dash: method-dash(method),
    ))
    .join("")
  chart-image(
    height: chart-height + 30,
    (chart-axes, lines, response-legend()).map(str).join(""),
  )
}

// Axis bounds are deliberately fixed across targets: the two published impulse
// figures are meant to be read against each other, which only works while the
// frames are identical.
#let peak-aligned-impulse-chart(rows, target: "crossover") = {
  let visible = rows.filter(row => {
    let sample = int(row.at("peak_aligned_index"))
    row.at("target") == target and sample >= -24 and sample <= 48
  })
  let x-pos(value) = x-linear(value, -24, 48)
  let y-pos(value) = y-linear(value, -80, 0)
  let chart-axes = axes(
    ((-24, "-24"), (-16, "-16"), (0, "0"), (16, "16"), (32, "32"), (48, "48")),
    ((-80, "-80"), (-60, "-60"), (-40, "-40"), (-20, "-20"), (0, "0")),
    x-pos,
    y-pos,
    "Sample relative to peak",
    "Relative coefficient magnitude (dB)",
  )
  let methods = (
    "budde-iterative",
    "budde-adaptive",
    "phase-interpolation",
    "complex-minimax",
    "minphase-truncation",
    "low-group-delay",
  )
  let lines = methods
    .map(method => impulse-polyline(
      visible.filter(row => row.at("method") == method),
      x-pos,
      y-pos,
      method-colors.at(method-index(method)),
      dash: method-dash(method),
    ))
    .join("")
  chart-image(
    height: chart-height + 30,
    (chart-axes, lines, response-legend()).map(str).join(""),
  )
}

#let representative-results-table(rows) = {
  let representative = rows.filter(row => row.at("target") == "parametric-eq")
  let cells = representative
    .map(row => (
      [#method-label(row.at("method"))],
      [#number(100 * float(row.at("relative_magnitude_error")), digits: 4)],
      [#number(float(row.at("rms_magnitude_error_db")), digits: 3)],
      [#number(float(row.at("mean_group_delay")), digits: 2)],
      [#number(100 * float(row.at("pre_peak_energy_ratio")), digits: 3)],
      [#row.at("iterations")],
    ))
    .flatten()
  table(
    columns: (1.5fr, 0.7fr, 0.7fr, 0.65fr, 0.65fr, 0.45fr),
    align: (left, right, right, right, right, right),
    table.header(
      [Method], [Rel. err.\ (%)], [RMS (dB)], [Delay], [Pre-peak (\%)], [$P$]
    ),
    ..cells,
  )
}

// The summary table is transposed relative to the 2012 layout: methods run down
// the rows and criteria across the columns. Six methods will not fit as six
// columns inside one text column, and the criterion names abbreviate far better
// than the method names do.
#let summary-methods = (
  ("budde-iterative", [Alternating, $d = 16$]),
  ("budde-adaptive", [Alternating, selected $d$]),
  ("phase-interpolation", [Phase interpolation]),
  ("complex-minimax", [Complex minimax]),
  ("minphase-truncation", [Minimum-phase truncation]),
  ("low-group-delay", [Low group delay]),
)

#let summary-criteria = (
  ([Rel. error], "relative_magnitude_error"),
  ([RMS dB], "rms_magnitude_error_db"),
  ([Mean delay], "mean_group_delay"),
  ([Pre-peak], "pre_peak_energy_ratio"),
  ([Coef. range], "coefficient_range_db"),
  ([Ripple], "group_delay_ripple"),
)

#let cross-target-summary-table(rows) = {
  let win(method, key) = cross-target-win-count(rows, method, key)
  let numeric = range(summary-criteria.len())
  table(
    columns: (1.45fr,) + numeric.map(_ => 0.52fr),
    align: (left,) + numeric.map(_ => center),
    table.header(
      [Method],
      ..summary-criteria.map(criterion => criterion.at(0)),
    ),
    ..summary-methods
      .map(method => (
        method.at(1),
        ..summary-criteria.map(criterion => [
          #win(method.at(0), criterion.at(1))
        ]),
      ))
      .flatten(),
  )
}

// The sweep charts read docs/reference-delay-sweep.csv, whose fixtures are
// 2049-tap prototypes rather than the 257-tap ones behind every other figure.
// The two must not be read against each other, which is why the sweep carries a
// prototype_taps column and why these captions state the fixture length.

#let sweep-targets = (
  ("low-pass", "First-order low-pass"),
  ("parametric-eq", "Parametric EQ"),
  ("crossover", "LR4 crossover"),
  ("deep-notch", "Deep notch"),
  ("room-correction", "Room correction"),
  ("steep-crossover", "LR8 crossover"),
)

#let sweep-target-index(target) = {
  let found = sweep-targets.position(entry => entry.at(0) == target)
  if found == none { 0 } else { found }
}

// sweep-target-legend labels by target rather than by method, because both
// series in these charts are the same design code at different budgets.
#let sweep-target-legend(y: 396) = {
  sweep-targets
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let target = pair.at(1).at(0)
      let x = plot-left + calc.rem(index, 3) * 195
      let row-y = y + calc.floor(index / 3) * 28
      (
        svg-line(
          x,
          row-y - 5,
          x + 26,
          row-y - 5,
          stroke: method-colors.at(sweep-target-index(target)),
          width: 3,
        ),
        svg-text(x + 33, row-y, pair.at(1).at(1), anchor: "start", size: 16),
      )
        .map(str)
        .join("")
    })
    .join("")
}

// latency-accuracy-chart is the headline comparison: what latency a
// linear-phase filter needs to reach the accuracy a minimum-phase-led design
// reaches almost immediately.
//
// Each curve is one target's linear-phase family, magnitude error against its
// own latency. Each filled circle is the same target's 1025-tap design at a zero
// budget. The horizontal distance from a circle to its own curve at equal height
// is the latency the linear-phase route has to spend to catch up.
#let latency-accuracy-chart(rows) = {
  // Both axes are logarithmic and both are clamped. The errors span fifteen
  // decades and this chart draws no clip path, so an unclamped point renders
  // outside the frame rather than disappearing.
  let x-min = 0.4
  let x-max = 600
  // The floor is the matching tolerance rather than the smallest value present.
  // The mixed-phase designs reach 1e-15 dB, so any floor clamps them; putting it
  // at the tolerance the comparison actually uses means a clamped circle and the
  // curve that meets it are being read at the same accuracy.
  let y-min = 0.0001
  let y-max = 100
  let match-tolerance = 0.001
  let x-pos(value) = x-log(calc.clamp(value, x-min, x-max), x-min, x-max)
  let y-pos(value) = y-log(calc.clamp(value, y-min, y-max), y-min, y-max)
  let chart-axes = axes(
    ((0.4, "0.4"), (1, "1"), (10, "10"), (100, "100"), (600, "600")),
    (
      (0.0001, "1e-4"),
      (0.001, "1e-3"),
      (0.01, "0.01"),
      (1, "1"),
      (100, "100"),
    ),
    x-pos,
    y-pos,
    "Latency: weighted mean group delay (samples)",
    "RMS magnitude error (dB)",
  )
  let curves = sweep-targets
    .map(entry => {
      let target = entry.at(0)
      let family = rows
        .filter(row => (
          row.at("target") == target and row.at("method") == "linear-phase"
        ))
        .sorted(key: row => float(row.at("mean_group_delay")))
      xy-polyline(
        family,
        x-pos,
        y-pos,
        "mean_group_delay",
        "rms_magnitude_error_db",
        method-colors.at(sweep-target-index(target)),
        dash: "7 5",
      )
    })
    .join("")
  let points = sweep-targets
    .map(entry => {
      let target = entry.at(0)
      let matches = rows.filter(row => (
        row.at("target") == target
          and row.at("method") == "mixed-phase"
          and row.at("taps") == "1025"
          and row.at("phase_delay_samples") == "0"
      ))
      if matches.len() == 0 {
        ""
      } else {
        let row = matches.first()
        let x = x-pos(float(row.at("mean_group_delay")))
        let y = y-pos(float(row.at("rms_magnitude_error_db")))
        (
          "<circle cx=\"",
          number(x),
          "\" cy=\"",
          number(y),
          "\" r=\"9\" fill=\"",
          method-colors.at(sweep-target-index(target)),
          "\" stroke=\"",
          ink,
          "\" stroke-width=\"2\"/>",
        )
          .map(str)
          .join("")
      }
    })
    .join("")
  let tolerance-line = (
    svg-line(
      plot-left,
      y-pos(match-tolerance),
      plot-right,
      y-pos(match-tolerance),
      stroke: muted,
      width: 2,
      dash: "3 4",
    ),
    svg-text(
      plot-left + 8,
      y-pos(match-tolerance) - 8,
      "matching tolerance",
      anchor: "start",
      size: 15,
      fill: muted,
    ),
  )
    .map(str)
    .join("")
  chart-image(
    height: chart-height + 20,
    (chart-axes, tolerance-line, curves, points, sweep-target-legend())
      .map(str)
      .join(""),
  )
}

// phase-regime-chart is the paper's central contrast: what a filter buys by
// spending latency above the group-delay floor its magnitude request implies.
//
// Both axes are normalised per target so that six targets share one frame.
// Latency is the fraction of the way from the target's own minimum-phase floor
// to linear phase, and ripple is relative to that target's own minimum-phase
// ripple. Both families therefore start at exactly (0, 1) and the linear-phase
// endpoint is exactly (1, 0), so the curves between them are directly readable
// as how the surplus was spent.
//
// The continuum collapses onto the descending diagonal: prescribing a phase
// between the endpoints converts latency into flatness in proportion. The
// alternating factorisation instead holds its ripple while its minimum-phase
// factor still fits its share of the taps, and falls only once the budget has
// starved that factor away, which costs the magnitude the design was for.
#let phase-regime-chart(rows) = {
  let y-max = 1.25
  let x-pos(value) = x-linear(calc.clamp(value, 0, 1), 0, 1)
  let y-pos(value) = y-linear(calc.clamp(value, 0, y-max), 0, y-max)
  let chart-axes = axes(
    ((0, "0"), (0.25, "0.25"), (0.5, "0.5"), (0.75, "0.75"), (1, "1")),
    ((0, "0"), (0.25, "0.25"), (0.5, "0.5"), (0.75, "0.75"), (1, "1")),
    x-pos,
    y-pos,
    "Latency above the floor (fraction to linear phase)",
    "Ripple, relative to minimum phase",
  )
  // Each target's own floor and its own minimum-phase ripple set the scales.
  let reference-of(target) = {
    let anchors = rows.filter(row => (
      row.at("target") == target and row.at("regime") == "continuum"
    ))
    let at-mix(mix) = {
      let found = anchors.filter(row => float(row.at("phase_mix")) == mix)
      if found.len() == 0 { none } else { found.first() }
    }
    let minimum = at-mix(0)
    let linear = at-mix(1)
    if minimum == none or linear == none {
      none
    } else {
      (
        floor: float(minimum.at("mean_group_delay")),
        span: float(linear.at("mean_group_delay"))
          - float(minimum.at("mean_group_delay")),
        ripple: float(minimum.at("group_delay_ripple")),
      )
    }
  }
  let family(target, regime, dash) = {
    let scale = reference-of(target)
    if scale == none or scale.span <= 0 or scale.ripple <= 0 {
      ""
    } else {
      let points = rows
        .filter(row => (
          row.at("target") == target and row.at("regime") == regime
        ))
        .map(row => (
          x: (float(row.at("mean_group_delay")) - scale.floor) / scale.span,
          y: float(row.at("group_delay_ripple")) / scale.ripple,
        ))
        // The continuum runs on past linear phase to maximum phase, which is
        // the mirror image of its first half and would double every curve
        // back over itself.
        .filter(point => point.x <= 1.0001)
        .sorted(key: point => point.x)
      xy-polyline(
        points.map(point => (
          mean_group_delay: str(point.x),
          group_delay_ripple: str(point.y),
        )),
        x-pos,
        y-pos,
        "mean_group_delay",
        "group_delay_ripple",
        method-colors.at(sweep-target-index(target)),
        width: 3,
        dash: dash,
      )
    }
  }
  let continuum = sweep-targets
    .map(entry => family(entry.at(0), "continuum", none))
    .join("")
  let factorisation = sweep-targets
    .map(entry => family(entry.at(0), "factorisation", "7 5"))
    .join("")
  // A style key, since colour already carries the target.
  let style-key = {
    let y = plot-top + 26
    (
      svg-line(plot-right - 232, y - 5, plot-right - 206, y - 5, width: 3),
      svg-text(
        plot-right - 199,
        y,
        "prescribed continuum",
        anchor: "start",
        size: 16,
      ),
      svg-line(
        plot-right - 232,
        y + 23,
        plot-right - 206,
        y + 23,
        width: 3,
        dash: "7 5",
      ),
      svg-text(
        plot-right - 199,
        y + 28,
        "alternating factorisation",
        anchor: "start",
        size: 16,
      ),
    )
      .map(str)
      .join("")
  }
  chart-image(
    height: chart-height + 20,
    (chart-axes, continuum, factorisation, style-key, sweep-target-legend())
      .map(str)
      .join(""),
  )
}

// ---------------------------------------------------------------------------
// Continuum charts
//
// These read docs/reference-continuum.csv and
// docs/reference-continuum-impulse.csv, whose rows are one requested group delay
// each rather than one method each. The shared x axis is therefore the knob
// itself, in samples, and every chart below spans the full causal range [0, N-1]
// so that the three regimes stay in register from figure to figure.
// ---------------------------------------------------------------------------

#let target-labels = (
  "low-pass": "Low-pass",
  "parametric-eq": "Parametric EQ",
  "crossover": "LR4 crossover",
  "deep-notch": "Deep notch",
  "room-correction": "Room correction",
  "steep-crossover": "LR8 crossover",
)

#let target-index(target) = {
  let found = reference-targets.position(name => name == target)
  if found == none { 0 } else { found }
}

#let target-color(target) = method-colors.at(target-index(target))

#let target-dash(target) = (
  none,
  "13 7",
  "3 6",
  "2 4",
  "13 5 3 5",
  "7 4 2 4",
).at(target-index(target))

#let target-legend(y: 396) = {
  reference-targets
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let target = pair.at(1)
      let x = plot-left + calc.rem(index, 2) * 292
      let row-y = y + calc.floor(index / 2) * 27
      (
        svg-line(
          x,
          row-y - 5,
          x + 34,
          row-y - 5,
          stroke: target-color(target),
          width: 4,
          dash: target-dash(target),
        ),
        svg-text(
          x + 44,
          row-y,
          target-labels.at(target),
          anchor: "start",
          size: 16,
        ),
      )
        .map(str)
        .join("")
    })
    .join("")
}

// The causal range of a 129-tap filter. Every continuum chart uses it, so the
// figures can be read against one another and against the linear-phase mark at
// its centre.
#let continuum-span = 128
#let continuum-x-ticks = (
  (0, "0"),
  (32, "32"),
  (64, "64"),
  (96, "96"),
  (128, "128"),
)

#let continuum-rows-for(rows, target, regime) = rows.filter(row => (
  row.at("target") == target and row.at("regime") == regime
))

// continuum-window-rows returns one target's in-window rows, which are the only
// ones carrying a mix and a prediction.
#let continuum-window-rows(rows, target) = continuum-rows-for(
  rows,
  target,
  "window",
)

// continuum-branch marks the two optimised branches, which are sampled too
// sparsely to draw as a line and are the interesting points anyway.
#let continuum-branch-markers(
  rows,
  target,
  x-pos,
  y-pos,
  y-key,
  fill: auto,
) = {
  let color = if fill == auto { target-color(target) } else { fill }
  ("sub-minimum", "super-maximum")
    .map(regime => {
      continuum-rows-for(rows, target, regime)
        .map(row => {
          let x = x-pos(float(row.at("requested_delay")))
          let y = y-pos(float(row.at(y-key)))
          (
            "<circle cx=\"",
            number(x),
            "\" cy=\"",
            number(y),
            "\" r=\"4.5\" fill=\"",
            color,
            "\" stroke=\"",
            ink,
            "\" stroke-width=\"1\"/>",
          )
            .map(str)
            .join("")
        })
        .join("")
    })
    .join("")
}

// linear-phase-mark draws the dotted vertical at (N-1)/2, the one delay at which
// group delay is exactly flat.
#let linear-phase-mark(x-pos, label-y: plot-top + 16) = {
  (
    svg-line(
      x-pos(continuum-span / 2),
      plot-top,
      x-pos(continuum-span / 2),
      plot-bottom,
      stroke: muted,
      width: 2,
      dash: "5 5",
    ),
    svg-text(
      x-pos(continuum-span / 2) + 8,
      label-y,
      "linear phase",
      anchor: "start",
      size: 15,
      fill: muted,
    ),
  )
    .map(str)
    .join("")
}

// continuum-ripple-chart shows the quantity the knob actually shapes. Only the
// in-window rows are drawn: below the floor the ripple runs to 27 samples and
// would flatten everything else against the axis.
#let continuum-ripple-chart(rows) = {
  let x-pos(value) = x-linear(value, 0, continuum-span)
  let y-max = 7.5
  let y-pos(value) = y-linear(calc.clamp(value, 0, y-max), 0, y-max)
  let curves = reference-targets
    .map(target => xy-polyline(
      continuum-window-rows(rows, target),
      x-pos,
      y-pos,
      "requested_delay",
      "group_delay_ripple",
      target-color(target),
      dash: target-dash(target),
    ))
    .join("")
  let chart-axes = axes(
    continuum-x-ticks,
    ((0, "0"), (1.5, "1.5"), (3, "3"), (4.5, "4.5"), (6, "6"), (7.5, "7.5")),
    x-pos,
    y-pos,
    "Requested group delay (samples)",
    "Group-delay ripple (samples)",
  )
  chart-image(
    (chart-axes, curves, linear-phase-mark(x-pos), target-legend())
      .map(str)
      .join(""),
    height: 500,
  )
}

// continuum-accuracy-chart is the counter-intuitive one: the phase-pure ends of
// the continuum are the accurate places, and both the interior and the two
// optimised branches cost magnitude. The vertical scale is ten decades because
// the endpoints reach 1.6e-10 while the sub-floor points sit near 1e-1.
#let continuum-accuracy-chart(rows) = {
  let x-pos(value) = x-linear(value, 0, continuum-span)
  let y-min = 0.0000000001
  let y-max = 1
  let y-pos(value) = y-log(calc.clamp(value, y-min, y-max), y-min, y-max)
  let curves = reference-targets
    .map(target => xy-polyline(
      continuum-window-rows(rows, target),
      x-pos,
      y-pos,
      "requested_delay",
      "relative_magnitude_error",
      target-color(target),
      dash: target-dash(target),
    ))
    .join("")
  let branches = reference-targets
    .map(target => continuum-branch-markers(
      rows,
      target,
      x-pos,
      y-pos,
      "relative_magnitude_error",
    ))
    .join("")
  let chart-axes = axes(
    continuum-x-ticks,
    (
      (0.0000000001, "1e-10"),
      (0.00000001, "1e-8"),
      (0.000001, "1e-6"),
      (0.0001, "1e-4"),
      (0.01, "1e-2"),
      (1, "1"),
    ),
    x-pos,
    y-pos,
    "Requested group delay (samples)",
    "Relative magnitude error",
  )
  chart-image(
    (chart-axes, curves, branches, linear-phase-mark(x-pos), target-legend())
      .map(str)
      .join(""),
    height: 500,
  )
}

// continuum-window-chart is the diagnostic figure: how much phase freedom each
// requested magnitude leaves in the same 129 taps. The bar is the reachable
// window and the shaded ends are the regions only a magnitude concession opens.
#let continuum-window-chart(rows) = {
  let x-pos(value) = x-linear(value, 0, continuum-span)
  let lane-height = (plot-bottom - plot-top) / (reference-targets.len() + 1)
  let lanes = reference-targets
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let target = pair.at(1)
      let target-rows = rows.filter(row => row.at("target") == target)
      let floor = float(target-rows.at(0).at("minimum_phase_delay"))
      let y = plot-top + lane-height * (index + 0.5)
      (
        svg-line(
          x-pos(0),
          y,
          x-pos(continuum-span),
          y,
          stroke: grid-color,
          width: 12,
        ),
        svg-line(
          x-pos(floor),
          y,
          x-pos(continuum-span - floor),
          y,
          stroke: target-color(target),
          width: 12,
        ),
        svg-text(
          x-pos(0) + 4,
          y - 13,
          target-labels.at(target),
          anchor: "start",
          size: 16,
        ),
        svg-text(
          x-pos(continuum-span) - 4,
          y - 13,
          (
            number(continuum-span - 2 * floor, digits: 1),
            " samples wide",
          )
            .map(str)
            .join(""),
          anchor: "end",
          size: 16,
          fill: muted,
        ),
      )
        .map(str)
        .join("")
    })
    .join("")
  let chart-axes = axes(
    continuum-x-ticks,
    (),
    x-pos,
    y-pos => y-pos,
    "Group delay (samples)",
    "",
  )
  chart-image(
    (
      chart-axes,
      lanes,
      linear-phase-mark(x-pos, label-y: plot-bottom - 8),
    )
      .map(str)
      .join(""),
    height: 400,
  )
}

// continuum-residual-chart isolates the error of the affine delay law, which is
// what decides whether a requested delay can be met in closed form. The law is
// exact for the prescribed phase, so everything plotted here is the projection
// onto a finite support.
#let continuum-residual-chart(rows) = {
  let x-pos(value) = x-linear(value, 0, continuum-span)
  let y-max = 0.3
  let y-pos(value) = y-linear(calc.clamp(value, -y-max, y-max), -y-max, y-max)
  let zero = svg-line(
    x-pos(0),
    y-pos(0),
    x-pos(continuum-span),
    y-pos(0),
    stroke: grid-color,
    width: 4,
  )
  let curves = reference-targets
    .map(target => {
      let residual-rows = continuum-window-rows(rows, target).map(row => (
        "requested_delay": row.at("requested_delay"),
        "residual": str(
          float(row.at("mean_group_delay")) - float(row.at("predicted_delay")),
        ),
      ))
      xy-polyline(
        residual-rows,
        x-pos,
        y-pos,
        "requested_delay",
        "residual",
        target-color(target),
        dash: target-dash(target),
      )
    })
    .join("")
  let chart-axes = axes(
    continuum-x-ticks,
    (
      (-0.3, "-0.3"),
      (-0.15, "-0.15"),
      (0, "0"),
      (0.15, "0.15"),
      (0.3, "0.3"),
    ),
    x-pos,
    y-pos,
    "Requested group delay (samples)",
    "Delay residual (samples)",
  )
  chart-image(
    (chart-axes, zero, curves, linear-phase-mark(x-pos), target-legend())
      .map(str)
      .join(""),
    height: 500,
  )
}

// continuum-comparison-chart puts the knob's own curve next to the fixed points
// the other methods reach on the same target, on the axes a latency-constrained
// caller cares about: delay against magnitude error.
//
// The continuum is a curve because its delay is an input; every other method
// here reports whatever delay its own parameter produced, so each is one point.
#let continuum-comparison-chart(
  results,
  continuum,
  target: "low-pass",
  x-max: 128,
  y-min: 0.0000000001,
) = {
  let x-pos(value) = x-linear(value, 0, x-max)
  let y-max = 1
  let y-pos(value) = y-log(calc.clamp(value, y-min, y-max), y-min, y-max)
  let curve = xy-polyline(
    continuum-window-rows(continuum, target),
    x-pos,
    y-pos,
    "requested_delay",
    "relative_magnitude_error",
    ink,
    width: 4,
  )
  // Black, matching the continuum curve: the coloured method markers include a
  // filled circle, so a target-coloured branch point would be unreadable here.
  let branches = continuum-branch-markers(
    continuum,
    target,
    x-pos,
    y-pos,
    "relative_magnitude_error",
    fill: ink,
  )
  let points = results
    .filter(row => row.at("target") == target)
    .map(row => marker(
      row.at("method"),
      x-pos(float(row.at("mean_group_delay"))),
      y-pos(float(row.at("relative_magnitude_error"))),
    ))
    .join("")
  let decades = (
    (0.0000000001, "1e-10"),
    (0.00000001, "1e-8"),
    (0.000001, "1e-6"),
    (0.0001, "1e-4"),
    (0.01, "1e-2"),
    (1, "1"),
  ).filter(tick => tick.at(0) >= y-min)
  let x-ticks = continuum-x-ticks.filter(tick => tick.at(0) <= x-max)
  let chart-axes = axes(
    x-ticks,
    decades,
    x-pos,
    y-pos,
    "Mean group delay (samples)",
    "Relative magnitude error",
  )
  chart-image(
    (chart-axes, curve, branches, points, method-legend()).map(str).join(""),
    height: 500,
  )
}

// continuum-impulse-chart draws the impulse response at each sampled position of
// one target's continuum, stacked in one frame with a shared amplitude scale.
//
// The point of the figure is the last lane: it is the first lane read backwards.
#let continuum-impulse-chart(rows, target: "low-pass") = {
  let target-rows = rows.filter(row => row.at("target") == target)
  let delays = target-rows
    .filter(row => int(row.at("sample_index")) == 0)
    .map(row => row.at("requested_delay"))
  let taps = if target-rows.len() == 0 {
    1
  } else {
    int(target-rows.at(0).at("taps"))
  }
  let x-pos(value) = x-linear(value, 0, taps - 1)
  let lane-height = (plot-bottom - plot-top) / delays.len()
  let lanes = delays
    .enumerate()
    .map(pair => {
      let index = pair.at(0)
      let delay = pair.at(1)
      let lane-rows = target-rows.filter(row => (
        row.at("requested_delay") == delay
      ))
      let regime = lane-rows.at(0).at("regime")
      let centre = plot-top + lane-height * (index + 0.5)
      let amplitude = lane-height * 0.42
      let y-pos(value) = centre - value * amplitude
      let baseline = svg-line(
        x-pos(0),
        centre,
        x-pos(taps - 1),
        centre,
        stroke: grid-color,
        width: 1,
      )
      let stems = lane-rows
        .map(row => svg-line(
          x-pos(float(row.at("sample_index"))),
          centre,
          x-pos(float(row.at("sample_index"))),
          y-pos(float(row.at("normalised_coefficient"))),
          stroke: target-color(target),
          width: 1.6,
        ))
        .join("")
      let caption = (
        number(float(delay), digits: 1),
        " samples · ",
        regime,
      )
        .map(str)
        .join("")
      (
        baseline,
        stems,
        svg-text(
          plot-right - 12,
          centre - amplitude + 2,
          caption,
          anchor: "end",
          size: 15,
          fill: muted,
        ),
      )
        .map(str)
        .join("")
    })
    .join("")
  let chart-axes = axes(
    ((0, "0"), (32, "32"), (64, "64"), (96, "96"), (128, "128")),
    (),
    x-pos,
    y-pos => y-pos,
    "Sample index",
    "",
  )
  chart-image((chart-axes, lanes).map(str).join(""), height: 400)
}

// continuum-summary-table reports, per target, the two quantities that decide
// what the knob can do for it: the floor its magnitude implies, and how much
// accuracy the phase choice costs between the ends of its window.
//
// Every cell is computed from docs/reference-continuum.csv at build time, so the
// table cannot drift from the artifact the figures draw.
#let continuum-summary-table(rows) = {
  let entry(target) = {
    let window-rows = continuum-window-rows(rows, target)
    let floor = float(window-rows.at(0).at("minimum_phase_delay"))
    let endpoint = float(window-rows.at(0).at("relative_magnitude_error"))
    let interior = window-rows
      .slice(1, window-rows.len() - 1)
      .map(row => float(row.at("relative_magnitude_error")))
    let peak = interior.fold(0.0, (best, value) => calc.max(best, value))
    (
      floor: floor,
      width: continuum-span - 2 * floor,
      endpoint: endpoint,
      peak: peak,
      ratio: peak / endpoint,
    )
  }
  let exponential(value, digits: 1) = {
    if value == 0 {
      "0"
    } else {
      let power = calc.floor(log10(value))
      let mantissa = value / calc.pow(10.0, power)
      (number(mantissa, digits: digits), "e", str(power)).map(str).join("")
    }
  }
  table(
    columns: (1.42fr, 0.56fr, 0.54fr, 0.72fr, 0.72fr, 0.6fr),
    align: (left, right, right, right, right, right),
    table.header(
      [Target], [$tau_"min"$], [Width], [$E_"end"$], [$E_"peak"$], [Ratio]
    ),
    ..reference-targets
      .map(target => {
        let values = entry(target)
        (
          target-labels.at(target),
          number(values.floor, digits: 2),
          number(values.width, digits: 1),
          exponential(values.endpoint),
          exponential(values.peak),
          exponential(values.ratio),
        )
      })
      .flatten()
      .map(cell => [#cell]),
  )
}

// continuum-subfloor-table reports the exchange below the floor: what fraction
// of its floor each target gives up, and what the magnitude paid for it.
//
// The row chosen per target is the most aggressive request the artifact samples,
// so the table is the worst case of the ladder rather than an average of it.
#let continuum-subfloor-table(rows) = {
  table(
    columns: (1.2fr, 0.58fr, 0.54fr, 0.74fr, 0.7fr, 0.64fr),
    align: (left, right, right, right, right, right),
    table.header(
      [Target], [$tau_"min"$], [$tau$], [$E_"rel"$ (%)], [RMS (dB)], [Ripple]
    ),
    ..reference-targets
      .map(target => {
        let sub = continuum-rows-for(rows, target, "sub-minimum")
        let row = sub.at(0)
        let floor = float(row.at("minimum_phase_delay"))
        let achieved = float(row.at("mean_group_delay"))
        (
          target-labels.at(target),
          number(floor, digits: 2),
          number(achieved, digits: 2),
          number(100 * float(row.at("relative_magnitude_error")), digits: 1),
          number(float(row.at("rms_magnitude_error_db")), digits: 1),
          number(float(row.at("group_delay_ripple")), digits: 1),
        )
      })
      .flatten()
      .map(cell => [#cell]),
  )
}
