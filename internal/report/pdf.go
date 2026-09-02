package report

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/go-pdf/fpdf"
)

// Palette mirrors web/src/lib/status/tokens.ts's "-fg" tones so the PDF and
// the on-screen dashboard read as the same report. ISP severity additionally
// escalates HIGH_CONFIDENCE_BLOCK to rose (vs. SUSPECTED's violet) since a
// donut chart needs to distinguish the two, unlike the single-tone status
// badge convention used elsewhere in the app.
var (
	colorInk      = [3]int{15, 23, 42}    // slate-900, primary text
	colorMuted    = [3]int{100, 116, 139} // slate-500, secondary text
	colorGrid     = [3]int{226, 232, 240} // slate-200, borders/gridlines
	colorTileBg   = [3]int{248, 250, 252} // slate-50, tile fill
	colorEmerald  = [3]int{4, 120, 87}
	colorAmber    = [3]int{180, 83, 9}
	colorRose     = [3]int{190, 18, 60}
	colorViolet   = [3]int{109, 40, 217}
	colorSlateMid = [3]int{100, 116, 139}
)

func availabilityColor(status string) [3]int {
	switch status {
	case "ACTIVE":
		return colorEmerald
	case "DEGRADED":
		return colorAmber
	case "UNAVAILABLE":
		return colorRose
	default:
		return colorSlateMid
	}
}

func ispColor(status string) [3]int {
	switch status {
	case "NOT_DETECTED":
		return colorEmerald
	case "SUSPECTED":
		return colorViolet
	case "HIGH_CONFIDENCE_BLOCK":
		return colorRose
	default:
		return colorSlateMid
	}
}

// renderDashboardPDF draws the same KPI/distribution/trend/top-domains data
// as the on-screen dashboard onto a single A4 page (fpdf auto-page-breaks if
// a future data shape overflows it). All drawing uses fpdf's own primitives
// (Polygon-approximated arcs for the donuts, Line for the trend chart) — no
// external chart or headless-browser dependency.
func renderDashboardPDF(dashboard Dashboard) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	drawDashboardHeader(pdf, dashboard)

	drawKPIRow(pdf, []kpiTile{
		{"Total domains", fmt.Sprint(dashboard.TotalDomains)},
		{"Active domains", fmt.Sprint(dashboard.ActiveDomains)},
		{"Expiring in 90 days", fmt.Sprint(dashboard.ExpiringSoon)},
		{"Renewal cost", formatMoney2dp(dashboard.RenewalCost) + " " + dashboard.ReportingCurrency},
	}, 15, 32, 180)
	drawKPIRow(pdf, []kpiTile{
		{"Unavailable domains", fmt.Sprint(dashboard.UnavailableDomains)},
		{"Suspected ISP block", fmt.Sprint(dashboard.SuspectedISPBlock)},
		{"DNS errors", fmt.Sprint(dashboard.DNSErrors)},
		{"TLS errors", fmt.Sprint(dashboard.TLSErrors)},
	}, 15, 52, 180)

	sectionY := 78.0
	pdf.SetFont("Helvetica", "B", 11)
	setTextColor(pdf, colorInk)
	pdf.SetXY(15, sectionY)
	pdf.CellFormat(85, 6, "Availability status", "", 0, "L", false, 0, "")
	pdf.SetXY(110, sectionY)
	pdf.CellFormat(85, 6, "ISP status", "", 0, "L", false, 0, "")

	donutY := sectionY + 24
	drawDonut(pdf, 40, donutY, 16, dashboard.AvailabilityDistribution, availabilityColor)
	drawDonut(pdf, 135, donutY, 16, dashboard.ISPDistribution, ispColor)
	drawLegend(pdf, 62, donutY-13, dashboard.AvailabilityDistribution, availabilityColor)
	drawLegend(pdf, 157, donutY-13, dashboard.ISPDistribution, ispColor)

	trendY := donutY + 26
	pdf.SetFont("Helvetica", "B", 11)
	setTextColor(pdf, colorInk)
	pdf.SetXY(15, trendY)
	pdf.CellFormat(180, 6, "Incidents opened per day (last 30 days)", "", 1, "L", false, 0, "")
	drawTrendChart(pdf, dashboard.IncidentTrend30d, 27, trendY+9, 165, 26)

	tableY := trendY + 44
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetXY(15, tableY)
	pdf.CellFormat(180, 6, "Domains needing attention (top 10)", "", 1, "L", false, 0, "")
	drawTopDomainsTable(pdf, dashboard.TopDomains, 15, tableY+8, 180)

	if len(dashboard.CompletenessWarnings) > 0 {
		pdf.SetFont("Helvetica", "", 7)
		setTextColor(pdf, colorAmber)
		pdf.SetXY(15, pdf.GetY()+4)
		pdf.MultiCell(180, 4, "Warnings: "+strings.Join(dashboard.CompletenessWarnings, "; "), "", "L", false)
	}

	buffer := &bytes.Buffer{}
	if err := pdf.Output(buffer); err != nil {
		return nil, fmt.Errorf("render dashboard pdf: %w", err)
	}
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("render dashboard pdf: %w", err)
	}
	return buffer.Bytes(), nil
}

func drawDashboardHeader(pdf *fpdf.Fpdf, dashboard Dashboard) {
	pdf.SetFont("Helvetica", "B", 18)
	setTextColor(pdf, colorInk)
	pdf.SetXY(15, 15)
	pdf.CellFormat(180, 9, "Domain Monitoring Report", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	setTextColor(pdf, colorMuted)
	pdf.SetX(15)
	subtitle := fmt.Sprintf("Generated %s UTC  |  Reporting currency: %s  |  Policy: %s",
		dashboard.GeneratedAt.Format("2 Jan 2006 15:04"), dashboard.ReportingCurrency, dashboard.RecommendationPolicy)
	pdf.CellFormat(180, 5, subtitle, "", 1, "L", false, 0, "")

	setDrawColor(pdf, colorGrid)
	pdf.SetLineWidth(0.3)
	pdf.Line(15, 28, 195, 28)
}

type kpiTile struct {
	Label string
	Value string
}

func drawKPIRow(pdf *fpdf.Fpdf, tiles []kpiTile, x, y, w float64) {
	const gap = 3.0
	tileW := (w - gap*float64(len(tiles)-1)) / float64(len(tiles))
	for i, tile := range tiles {
		tx := x + float64(i)*(tileW+gap)
		setFillColor(pdf, colorTileBg)
		setDrawColor(pdf, colorGrid)
		pdf.Rect(tx, y, tileW, 18, "FD")

		pdf.SetXY(tx+2.5, y+2.5)
		pdf.SetFont("Helvetica", "", 7)
		setTextColor(pdf, colorMuted)
		pdf.CellFormat(tileW-5, 4, tile.Label, "", 0, "L", false, 0, "")

		pdf.SetXY(tx+2.5, y+8)
		pdf.SetFont("Helvetica", "B", 13)
		setTextColor(pdf, colorInk)
		pdf.CellFormat(tileW-5, 8, tile.Value, "", 0, "L", false, 0, "")
	}
}

// drawDonut renders a pie-with-hole chart as a fan of filled polygon wedges
// (fpdf has no native arc-fill primitive), one per non-zero status, in
// descending count order — then punches the hole by overpainting a smaller
// filled circle in the page background color.
func drawDonut(pdf *fpdf.Fpdf, cx, cy, radius float64, counts []StatusCount, colorFn func(string) [3]int) {
	total := sumCounts(counts)
	if total == 0 {
		setFillColor(pdf, colorGrid)
		pdf.Circle(cx, cy, radius, "F")
		pdf.SetFillColor(255, 255, 255)
		pdf.Circle(cx, cy, radius*0.55, "F")
		return
	}
	angle := 0.0
	for _, item := range counts {
		if item.Count == 0 {
			continue
		}
		sweep := 360 * float64(item.Count) / float64(total)
		drawPieSlice(pdf, cx, cy, radius, angle, angle+sweep, colorFn(item.Status))
		angle += sweep
	}
	pdf.SetFillColor(255, 255, 255)
	pdf.Circle(cx, cy, radius*0.55, "F")
}

func drawPieSlice(pdf *fpdf.Fpdf, cx, cy, radius, startDeg, endDeg float64, rgb [3]int) {
	setFillColor(pdf, rgb)
	steps := int(math.Ceil((endDeg - startDeg) / 3))
	if steps < 1 {
		steps = 1
	}
	points := make([]fpdf.PointType, 0, steps+2)
	points = append(points, fpdf.PointType{X: cx, Y: cy})
	for i := 0; i <= steps; i++ {
		angle := startDeg + (endDeg-startDeg)*float64(i)/float64(steps)
		points = append(points, polarPoint(cx, cy, radius, angle))
	}
	pdf.Polygon(points, "F")
}

// polarPoint places angle 0 at 12 o'clock, sweeping clockwise — the usual
// donut-chart convention — in a coordinate system where y grows downward.
func polarPoint(cx, cy, radius, angleDeg float64) fpdf.PointType {
	rad := angleDeg * math.Pi / 180
	return fpdf.PointType{X: cx + radius*math.Sin(rad), Y: cy - radius*math.Cos(rad)}
}

func drawLegend(pdf *fpdf.Fpdf, x, y float64, counts []StatusCount, colorFn func(string) [3]int) {
	total := sumCounts(counts)
	pdf.SetFont("Helvetica", "", 8)
	for _, item := range counts {
		rgb := colorFn(item.Status)
		setFillColor(pdf, rgb)
		pdf.Rect(x, y+1, 3, 3, "F")
		percentage := 0.0
		if total > 0 {
			percentage = 100 * float64(item.Count) / float64(total)
		}
		pdf.SetXY(x+5, y)
		setTextColor(pdf, colorInk)
		label := fmt.Sprintf("%s: %d (%.0f%%)", humanizeStatus(item.Status), item.Count, percentage)
		pdf.CellFormat(70, 5, label, "", 0, "L", false, 0, "")
		y += 5.5
	}
}

func drawTrendChart(pdf *fpdf.Fpdf, trend []DailyCount, x, y, w, h float64) {
	setDrawColor(pdf, colorGrid)
	pdf.SetLineWidth(0.2)
	pdf.Line(x, y+h, x+w, y+h)
	pdf.Line(x, y, x, y+h)
	if len(trend) == 0 {
		return
	}
	maxCount := 1
	for _, day := range trend {
		if day.Count > maxCount {
			maxCount = day.Count
		}
	}
	stepX := 0.0
	if len(trend) > 1 {
		stepX = w / float64(len(trend)-1)
	}
	setDrawColor(pdf, colorRose)
	pdf.SetLineWidth(0.5)
	prevX, prevY := x, y+h
	for i, day := range trend {
		px := x + stepX*float64(i)
		py := y + h - h*float64(day.Count)/float64(maxCount)
		if i > 0 {
			pdf.Line(prevX, prevY, px, py)
		}
		prevX, prevY = px, py
	}

	pdf.SetFont("Helvetica", "", 7)
	setTextColor(pdf, colorMuted)
	pdf.SetXY(x-13, y-2)
	pdf.CellFormat(11, 4, fmt.Sprint(maxCount), "", 0, "R", false, 0, "")
	pdf.SetXY(x-13, y+h-2)
	pdf.CellFormat(11, 4, "0", "", 0, "R", false, 0, "")
	pdf.SetXY(x, y+h+2)
	pdf.CellFormat(30, 4, trend[0].Date, "", 0, "L", false, 0, "")
	pdf.SetXY(x+w-30, y+h+2)
	pdf.CellFormat(30, 4, trend[len(trend)-1].Date, "", 0, "R", false, 0, "")
}

func drawTopDomainsTable(pdf *fpdf.Fpdf, domains []TopDomain, x, y, w float64) {
	colWidths := []float64{w * 0.32, w * 0.19, w * 0.19, w * 0.16, w * 0.14}
	headers := []string{"Domain", "Availability", "ISP status", "Recommendation", "Renewal cost"}

	pdf.SetFont("Helvetica", "B", 8)
	setFillColor(pdf, colorTileBg)
	setTextColor(pdf, colorInk)
	pdf.SetXY(x, y)
	for i, header := range headers {
		pdf.CellFormat(colWidths[i], 6, header, "B", 0, "L", true, 0, "")
	}
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "", 8)
	if len(domains) == 0 {
		pdf.SetX(x)
		setTextColor(pdf, colorMuted)
		pdf.CellFormat(w, 6, "No domains currently need attention.", "", 0, "L", false, 0, "")
		return
	}
	const cellPadding = 2.0
	for i, domain := range domains {
		fill := i%2 == 1
		setFillColor(pdf, colorTileBg)
		setTextColor(pdf, colorInk)
		pdf.SetX(x)
		renewal := "-"
		if domain.RenewalCost != nil && domain.RenewalCostCurrency != nil {
			renewal = formatMoney2dp(*domain.RenewalCost) + " " + *domain.RenewalCostCurrency
		}
		row := []string{domain.Domain, domain.AvailabilityStatus, domain.ISPStatus, domain.Recommendation, renewal}
		for c, cell := range row {
			pdf.CellFormat(colWidths[c], 6, truncateToWidth(pdf, cell, colWidths[c]-cellPadding), "B", 0, "L", fill, 0, "")
		}
		pdf.Ln(6)
	}
}

// truncateToWidth shortens text with a trailing ellipsis so it never spills
// past its table cell — fpdf's CellFormat does not wrap or clip on its own.
func truncateToWidth(pdf *fpdf.Fpdf, text string, maxWidth float64) string {
	if pdf.GetStringWidth(text) <= maxWidth {
		return text
	}
	const ellipsis = "..."
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if pdf.GetStringWidth(string(runes)+ellipsis) <= maxWidth {
			return string(runes) + ellipsis
		}
	}
	return ellipsis
}

func sumCounts(counts []StatusCount) int {
	total := 0
	for _, item := range counts {
		total += item.Count
	}
	return total
}

func humanizeStatus(status string) string {
	lower := strings.ToLower(strings.ReplaceAll(status, "_", " "))
	if lower == "" {
		return lower
	}
	return strings.ToUpper(lower[:1]) + lower[1:]
}

// formatMoney2dp rounds a backend decimal string to 2 fraction digits via
// big.Rat (exact decimal arithmetic), never float64/strconv.ParseFloat, so
// the PDF's display precision matches the web app's MoneyValue component
// without risking binary-floating-point rounding on large or precise
// backend decimals.
func formatMoney2dp(raw string) string {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok {
		return raw
	}
	return rat.FloatString(2)
}

func setTextColor(pdf *fpdf.Fpdf, rgb [3]int) { pdf.SetTextColor(rgb[0], rgb[1], rgb[2]) }
func setFillColor(pdf *fpdf.Fpdf, rgb [3]int) { pdf.SetFillColor(rgb[0], rgb[1], rgb[2]) }
func setDrawColor(pdf *fpdf.Fpdf, rgb [3]int) { pdf.SetDrawColor(rgb[0], rgb[1], rgb[2]) }
