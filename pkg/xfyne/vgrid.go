package xfyne

import (
    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/theme"
)

// GridCell pairs a Fyne CanvasObject with a column span (default 1).
type GridCell struct {
    Object  fyne.CanvasObject
    ColSpan int
}

// Span creates a GridCell that spans colSpan columns.
func Span(obj fyne.CanvasObject, colSpan int) GridCell {
    if colSpan < 1 {
        colSpan = 1
    }
    return GridCell{Object: obj, ColSpan: colSpan}
}

type placedCell struct {
    obj     fyne.CanvasObject
    row     int
    col     int
    colSpan int
}

// VariableGridLayout arranges items into len(ColWeights) columns
// with proportional column widths and per-cell column spans.
type VariableGridLayout struct {
    ColWeights []float32
    Cells      []GridCell
}

// NewVariableGrid builds a *fyne.Container using VariableGridLayout.
// Each item can be a fyne.CanvasObject (treated as ColSpan = 1),
// a GridCell, or a *GridCell.
func NewVariableGrid(weights []float32, items ...any) *fyne.Container {
    w := make([]float32, len(weights))
    for i, v := range weights {
        if v <= 0 {
            v = 1
        }
        w[i] = v
    }

    cells := make([]GridCell, 0, len(items))
    objects := make([]fyne.CanvasObject, 0, len(items))

    for _, item := range items {
        switch v := item.(type) {
        case GridCell:
            span := v.ColSpan
            if span < 1 {
                span = 1
            }
            cells = append(cells, GridCell{Object: v.Object, ColSpan: span})
            if v.Object != nil {
                objects = append(objects, v.Object)
            }
        case *GridCell:
            if v == nil {
                continue
            }
            span := v.ColSpan
            if span < 1 {
                span = 1
            }
            cells = append(cells, GridCell{Object: v.Object, ColSpan: span})
            if v.Object != nil {
                objects = append(objects, v.Object)
            }
        case fyne.CanvasObject:
            cells = append(cells, GridCell{Object: v, ColSpan: 1})
            if v != nil {
                objects = append(objects, v)
            }
        }
    }

    layout := &VariableGridLayout{
        ColWeights: w,
        Cells:      cells,
    }
    return container.New(layout, objects...)
}

func (g *VariableGridLayout) cols() int {
    return len(g.ColWeights)
}

func (g *VariableGridLayout) totalWeight() float32 {
    var sum float32
    for _, w := range g.ColWeights {
        sum += w
    }
    if sum == 0 {
        return 1
    }
    return sum
}

func (g *VariableGridLayout) placeCells() ([]placedCell, int) {
    cols := g.cols()
    if cols == 0 {
        return nil, 0
    }

    occupied := [][]bool{}
    ensureRow := func(r int) {
        for len(occupied) <= r {
            occupied = append(occupied, make([]bool, cols))
        }
    }

    var placed []placedCell
    curRow, curCol := 0, 0
    maxRowUsed := 0

    for _, cell := range g.Cells {
        if cell.Object == nil || !cell.Object.Visible() {
            continue
        }

        span := cell.ColSpan
        if span < 1 {
            span = 1
        }
        if span > cols {
            span = cols
        }

        for {
            ensureRow(curRow)
            if curCol+span > cols {
                curRow++
                curCol = 0
                continue
            }

            free := true
            for c := curCol; c < curCol+span; c++ {
                if occupied[curRow][c] {
                    free = false
                    break
                }
            }
            if free {
                break
            }
            curCol++
        }

        for c := curCol; c < curCol+span; c++ {
            occupied[curRow][c] = true
        }

        placed = append(placed, placedCell{
            obj:     cell.Object,
            row:     curRow,
            col:     curCol,
            colSpan: span,
        })

        if curRow+1 > maxRowUsed {
            maxRowUsed = curRow + 1
        }

        curCol += span
        if curCol >= cols {
            curRow++
            curCol = 0
        }
    }

    return placed, maxRowUsed
}

func (g *VariableGridLayout) spanWeight(startCol, span int) float32 {
    var w float32
    for c := startCol; c < startCol+span && c < len(g.ColWeights); c++ {
        w += g.ColWeights[c]
    }
    return w
}

func (g *VariableGridLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
    cols := g.cols()
    placed, numRows := g.placeCells()
    if cols == 0 || len(placed) == 0 {
        return fyne.NewSize(0, 0)
    }

    pad := theme.Padding()
    totalW := g.totalWeight()
    var requiredTotalWidth float32
    rowHeights := make([]float32, numRows)

    for _, p := range placed {
        minSz := p.obj.MinSize()

        if minSz.Height > rowHeights[p.row] {
            rowHeights[p.row] = minSz.Height
        }

        spannedPad := pad * float32(p.colSpan-1)
        cellUsableMinW := minSz.Width - spannedPad
        if cellUsableMinW < 0 {
            cellUsableMinW = 0
        }

        sw := g.spanWeight(p.col, p.colSpan)
        if sw > 0 {
            impliedGridWidth := (cellUsableMinW * (totalW / sw)) + pad*float32(cols-1)
            if impliedGridWidth > requiredTotalWidth {
                requiredTotalWidth = impliedGridWidth
            }
        }
    }

    var totalHeight float32
    for _, h := range rowHeights {
        totalHeight += h
    }
    if numRows > 1 {
        totalHeight += pad * float32(numRows-1)
    }

    return fyne.NewSize(requiredTotalWidth, totalHeight)
}

func (g *VariableGridLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
    cols := g.cols()
    placed, numRows := g.placeCells()
    if cols == 0 || len(placed) == 0 {
        return
    }

    pad := theme.Padding()
    totalPadX := pad * float32(cols-1)
    usableWidth := containerSize.Width - totalPadX
    if usableWidth < 0 {
        usableWidth = 0
    }

    totalW := g.totalWeight()
    colWidths := make([]float32, cols)
    colX := make([]float32, cols)

    var x float32
    for c := 0; c < cols; c++ {
        colWidths[c] = usableWidth * (g.ColWeights[c] / totalW)
        colX[c] = x
        x += colWidths[c] + pad
    }

    rowHeights := make([]float32, numRows)
    for _, p := range placed {
        h := p.obj.MinSize().Height
        if h > rowHeights[p.row] {
            rowHeights[p.row] = h
        }
    }

    rowY := make([]float32, numRows)
    var y float32
    for r := 0; r < numRows; r++ {
        rowY[r] = y
        y += rowHeights[r] + pad
    }

    for _, p := range placed {
        var w float32
        for c := p.col; c < p.col+p.colSpan; c++ {
            w += colWidths[c]
        }
        if p.colSpan > 1 {
            w += pad * float32(p.colSpan-1)
        }

        p.obj.Move(fyne.NewPos(colX[p.col], rowY[p.row]))
        p.obj.Resize(fyne.NewSize(w, rowHeights[p.row]))
    }
}
