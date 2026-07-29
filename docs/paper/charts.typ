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
  "#944d73",
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
} else {
  3
}

#let method-label(method) = if method == "budde-iterative" {
  "Alternating"
} else if method == "phase-interpolation" {
  "Phase interpolation"
} else if method == "complex-minimax" {
  "Complex minimax"
} else {
  "Low group delay"
}

#let reference-targets = (
  "low-pass",
  "parametric-eq",
  "crossover",
  "deep-notch",
  "room-correction",
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
} else {
  "13 5 3 5"
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
  } else {
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
    let points = rows
      .map(row => (
        number(x-position(float(row.at("peak_aligned_index")))),
        ",",
        number(y-position(float(row.at("normalised_coefficient")))),
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

#let chart-image(body, definitions: "") = {
  let svg = (
    "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 ",
    str(chart-width),
    " ",
    str(chart-height),
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

#let axes(
  x-ticks,
  y-ticks,
  x-position,
  y-position,
  x-label,
  y-label,
) = {
  let horizontal = y-ticks
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

#let method-legend(y: 405) = {
  let methods = (
    "budde-iterative",
    "phase-interpolation",
    "complex-minimax",
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

#let response-legend(include-target: false, y: 401) = {
  let entries = if include-target {
    (
      ("target", "Target"),
      ("budde-iterative", method-label("budde-iterative")),
      ("phase-interpolation", method-label("phase-interpolation")),
      ("complex-minimax", method-label("complex-minimax")),
      ("low-group-delay", method-label("low-group-delay")),
    )
  } else {
    (
      ("budde-iterative", method-label("budde-iterative")),
      ("phase-interpolation", method-label("phase-interpolation")),
      ("complex-minimax", method-label("complex-minimax")),
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
  let x-pos(value) = x-linear(value, 0, 30)
  let y-pos(value) = y-log(value, 0.00001, 100)
  let points = rows
    .map(row => {
      let delay = float(row.at("mean_group_delay"))
      let error = float(row.at("relative_magnitude_error")) * 100
      marker(row.at("method"), x-pos(delay), y-pos(error))
    })
    .join("")
  let chart-axes = axes(
    ((0, "0"), (10, "10"), (20, "20"), (30, "30")),
    (
      (0.00001, "0.00001"),
      (0.001, "0.001"),
      (0.1, "0.1"),
      (10, "10"),
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
  )
  let methods = (
    "budde-iterative",
    "phase-interpolation",
    "complex-minimax",
    "low-group-delay",
  )
  let y-pos(value) = y-linear(value, 0, 50)
  let group-width = (plot-right - plot-left) / 5
  let bar-width = 21
  let bars = targets
    .enumerate()
    .map(target-pair => {
      let target-index = target-pair.at(0)
      let target = target-pair.at(1).at(0)
      methods
        .enumerate()
        .map(method-pair => {
          let method-index = method-pair.at(0)
          let method = method-pair.at(1)
          let row = rows.find(row => (
            row.at("target") == target and row.at("method") == method
          ))
          let value = float(row.at("pre_peak_energy_ratio")) * 100
          let x = (
            plot-left
              + target-index * group-width
              + 15
              + method-index * (bar-width + 3)
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
            str(method-index),
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
      <rect width=\"8\" height=\"8\" fill=\"#ead5e1\"/>
      <path d=\"M0,0 L8,8 M8,0 L0,8\" stroke=\"#65334f\" stroke-width=\"1.4\"/>
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

#let magnitude-response-chart(rows) = {
  let visible = rows.filter(row => {
    let frequency = float(row.at("frequency_hz"))
    frequency >= 50 and frequency <= 20000
  })
  let x-pos(value) = x-log(value, 50, 20000)
  let y-pos(value) = y-linear(value, -4, 12)
  let chart-axes = axes(
    ((50, "50"), (100, "100"), (1000, "1k"), (10000, "10k"), (20000, "20k")),
    ((-4, "-4"), (0, "0"), (4, "4"), (8, "8"), (12, "12")),
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
    "phase-interpolation",
    "complex-minimax",
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
    (chart-axes, lines, target-line, response-legend(include-target: true))
      .map(str)
      .join(""),
  )
}

#let group-delay-response-chart(rows) = {
  let visible = rows.filter(row => float(row.at("delay_weight")) > 0)
  let x-pos(value) = x-linear(value, 1800, 5000)
  let y-pos(value) = y-linear(value, -40, 40)
  let chart-axes = axes(
    (
      (1800, "1.8k"),
      (2600, "2.6k"),
      (3400, "3.4k"),
      (4200, "4.2k"),
      (5000, "5k"),
    ),
    ((-40, "-40"), (-20, "-20"), (0, "0"), (20, "20"), (40, "40")),
    x-pos,
    y-pos,
    "Frequency (Hz)",
    "Group delay (samples)",
  )
  let methods = (
    "budde-iterative",
    "phase-interpolation",
    "complex-minimax",
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
    (chart-axes, lines, response-legend()).map(str).join(""),
  )
}

#let peak-aligned-impulse-chart(rows) = {
  let visible = rows.filter(row => {
    let sample = int(row.at("peak_aligned_index"))
    sample >= -16 and sample <= 48
  })
  let x-pos(value) = x-linear(value, -16, 48)
  let y-pos(value) = y-linear(value, -0.1, 1.05)
  let chart-axes = axes(
    ((-16, "-16"), (0, "0"), (16, "16"), (32, "32"), (48, "48")),
    ((-0.1, "-0.1"), (0, "0"), (0.5, "0.5"), (1, "1")),
    x-pos,
    y-pos,
    "Sample relative to peak",
    "Normalised coefficient",
  )
  let methods = (
    "budde-iterative",
    "phase-interpolation",
    "complex-minimax",
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

#let cross-target-summary-table(rows) = {
  let win(method, key) = cross-target-win-count(rows, method, key)
  table(
    columns: (1.45fr, 0.52fr, 0.52fr, 0.52fr, 0.52fr),
    align: (left, center, center, center, center),
    table.header(
      [Criterion],
      [Alternating],
      [Phase interpolation],
      [Complex minimax],
      [Low group delay],
    ),
    [Lowest relative magnitude error],
    [#win("budde-iterative", "relative_magnitude_error")],
    [#win("phase-interpolation", "relative_magnitude_error")],
    [#win("complex-minimax", "relative_magnitude_error")],
    [#win("low-group-delay", "relative_magnitude_error")],

    [Lowest RMS magnitude error],
    [#win("budde-iterative", "rms_magnitude_error_db")],
    [#win("phase-interpolation", "rms_magnitude_error_db")],
    [#win("complex-minimax", "rms_magnitude_error_db")],
    [#win("low-group-delay", "rms_magnitude_error_db")],

    [Lowest mean group delay],
    [#win("budde-iterative", "mean_group_delay")],
    [#win("phase-interpolation", "mean_group_delay")],
    [#win("complex-minimax", "mean_group_delay")],
    [#win("low-group-delay", "mean_group_delay")],

    [Least pre-peak energy],
    [#win("budde-iterative", "pre_peak_energy_ratio")],
    [#win("phase-interpolation", "pre_peak_energy_ratio")],
    [#win("complex-minimax", "pre_peak_energy_ratio")],
    [#win("low-group-delay", "pre_peak_energy_ratio")],

    [Smallest coefficient range],
    [#win("budde-iterative", "coefficient_range_db")],
    [#win("phase-interpolation", "coefficient_range_db")],
    [#win("complex-minimax", "coefficient_range_db")],
    [#win("low-group-delay", "coefficient_range_db")],
  )
}
