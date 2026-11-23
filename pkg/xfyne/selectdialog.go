// Package xfyne provides extended composite widgets and dialogs built on top of Fyne.
package xfyne

import (
    "os"
    "path/filepath"
    "sort"
    "strings"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/storage"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
    "github.com/terefang/gommons/pkg/xdg"
)

// FileTypeOption pairs a human-readable dropdown label with a storage.FileFilter.
type FileTypeOption struct {
    Label  string
    Filter storage.FileFilter
}

// ToolbarItem identifies an individual button in the SelectDialog toolbar.
type ToolbarItem int

const (
    // ToolbarUp navigates to the parent directory of CurrentPath.
    ToolbarUp ToolbarItem = iota
    // ToolbarHome navigates to the user's home directory.
    ToolbarHome
    // ToolbarNewFolder prompts for a name and creates a subdirectory in CurrentPath.
    ToolbarNewFolder
    // ToolbarGridView displays directory entries in a wrapped grid layout.
    ToolbarGridView
    // ToolbarListView displays directory entries in a vertical list layout.
    ToolbarListView
)

// ViewMode controls how items in CurrentPath are arranged in the main pane.
type ViewMode int

const (
    // ViewModeList renders directory entries in a vertical widget.List.
    ViewModeList ViewMode = iota
    // ViewModeGrid renders directory entries in a wrapped multi-column grid.
    ViewModeGrid
)

// SelectDialog represents a custom file and directory selection dialog with
// recent directories and recent files history lists.
//
// Embeds widget.BaseWidget and calls ExtendBaseWidget during construction.
type SelectDialog struct {
    widget.BaseWidget

    // Title is the window title displayed when Show opens the dialog.
    Title string

    // CurrentPath holds the active directory path for the selector.
    CurrentPath string

    // ViewMode controls whether the main directory pane uses list or grid layout.
    ViewMode ViewMode

    // ShowHidden controls whether hidden entries (starting with ".") are shown.
    ShowHidden bool

    // OnSelected is invoked with the chosen file or entry path when the
    // confirmation button is pressed, if non-nil.
    OnSelected func(string)

    // OnCancelled is invoked when the cancel button is pressed, if non-nil.
    OnCancelled func()

    // Items holds the entries (files and folders) displayed in the main pane.
    Items []string

    // ItemIsFolder maps entry names in Items to whether they represent a folder.
    ItemIsFolder map[string]bool

    // filter restricts which non-directory files are shown in the browser pane.
    // When nil, all files are shown.
    filter storage.FileFilter

    // typeFilters maps file type option labels to their corresponding FileFilter.
    typeFilters map[string]storage.FileFilter

    // viewContainer holds either currentList or currentGrid depending on ViewMode.
    viewContainer *fyne.Container

    // currentGrid renders directory items in wrapped grid mode.
    currentGrid *fyne.Container

    // fileInput holds the editable target file/item name field.
    fileInput *widget.Entry

    // typeSelect provides file type filter options in the footer.
    typeSelect *widget.Select

    // lookInSelect shows CurrentPath and ancestor paths up to the root directory.
    lookInSelect *widget.Select

    // currentList renders the items in the active directory in list mode.
    currentList *widget.List

    // recentDirsList renders RecentDirs in the right sidebar.
    recentDirsList *widget.List

    // recentFilesList renders RecentFiles in the right sidebar.
    recentFilesList *widget.List

    // window holds the dialog window created by Show.
    window fyne.Window

    // fileTypeOptions holds the selectable entries shown in typeSelect.
    fileTypeOptions []FileTypeOption
    upBtn           *widget.Button
    homeBtn         *widget.Button
    newFolderBtn    *widget.Button
    miscBtn         *SplitButton
    openBtn         *widget.Button
    PreferredDirs   []string
}

// AddFileTypeFilter registers a labeled file filter in typeSelect and refreshes
// the dropdown if it has already been created.
func (d *SelectDialog) AddFileTypeFilter(label string, filter storage.FileFilter) {
    if label == "" {
        return
    }

    for i, opt := range d.fileTypeOptions {
        if opt.Label == label {
            d.fileTypeOptions[i].Filter = filter
            d.syncTypeSelect(label)
            return
        }
    }

    d.fileTypeOptions = append(d.fileTypeOptions, FileTypeOption{
        Label:  label,
        Filter: filter,
    })
    d.syncTypeSelect(label)
}

// SetFilter sets the active storage.FileFilter, ensures a matching label is
// shown in typeSelect, reloads directory entries, and refreshes browser views.
func (d *SelectDialog) SetFilter(filter storage.FileFilter) {
    d.filter = filter

    matchedLabel := ""
    for _, opt := range d.fileTypeOptions {
        if opt.Filter == filter {
            matchedLabel = opt.Label
            break
        }
    }

    if matchedLabel == "" {
        matchedLabel = filterLabel(filter)
        d.fileTypeOptions = append(d.fileTypeOptions, FileTypeOption{
            Label:  matchedLabel,
            Filter: filter,
        })
    }

    d.syncTypeSelect(matchedLabel)

    if d.CurrentPath != "" {
        d.loadDirectoryEntries()
    }
    d.refreshBrowserViews()
}

// SetShowHidden controls whether hidden files and folders (names starting with
// ".") are displayed, reloading directory entries and refreshing browser views.
func (d *SelectDialog) SetShowHidden(show bool) {
    d.ShowHidden = show
    if d.CurrentPath != "" {
        d.loadDirectoryEntries()
    }
    d.refreshBrowserViews()
}

func isHiddenName(name string) bool {
    return strings.HasPrefix(name, ".")
}

func filterLabel(filter storage.FileFilter) string {
    if filter == nil {
        return "All Files (*.*)"
    }
    if extFilter, ok := filter.(*storage.ExtensionFileFilter); ok && len(extFilter.Extensions) > 0 {
        return "Filtered Files (" + strings.Join(extFilter.Extensions, ", ") + ")"
    }
    return "Custom Filter"
}

func (d *SelectDialog) syncTypeSelect(selectedLabel string) {
    if d.typeSelect == nil {
        return
    }

    labels := make([]string, len(d.fileTypeOptions))
    for i, opt := range d.fileTypeOptions {
        labels[i] = opt.Label
    }

    d.typeSelect.Options = labels
    if selectedLabel != "" {
        d.typeSelect.SetSelected(selectedLabel)
    }
    d.typeSelect.Refresh()
}

type AllFilesFilter struct{}

func (a AllFilesFilter) Matches(uri fyne.URI) bool {
    return true
}

// NewSelectDialog constructs and initializes a SelectDialog with the given
// window title, starting path, and selection/cancellation callbacks.
func NewSelectDialog(title, startPath string, allFiles bool, onSelected func(string), onCancelled func()) *SelectDialog {
    allLabel := "All Files (*.*)"

    d := &SelectDialog{
        Title:        title,
        CurrentPath:  startPath,
        ViewMode:     ViewModeList,
        ShowHidden:   false,
        OnSelected:   onSelected,
        OnCancelled:  onCancelled,
        ItemIsFolder: make(map[string]bool),
        filter:       nil,
        typeFilters:  nil,

        // Seed initial history and sample directory entries.
        Items: []string{},
    }
    d.fileInput = widget.NewEntry()
    d.openBtn = widget.NewButton("Open", func() {
        chosen := d.fileInput.Text
        if d.CurrentPath != "" && chosen != "" && !filepath.IsAbs(chosen) {
            chosen = filepath.Join(d.CurrentPath, chosen)
        }
        if d.OnSelected != nil {
            d.OnSelected(chosen)
        }
        if d.window != nil {
            d.window.Close()
        }
    })
    d.openBtn.Importance = widget.HighImportance
    if allFiles {
        d.AddFileTypeFilter(allLabel, &AllFilesFilter{})
    }
    d.ExtendBaseWidget(d)
    return d
}

// SetPath sets the active directory path and navigates to it, updating
// directory entries, history lists, lookInSelect, and toolbar states.
func (d *SelectDialog) SetPath(path string) {
    d.NavigateTo(path)
}

// Show creates and displays a new window for the dialog using fyne.CurrentApp().
// It is a no-op if no current Fyne application is available.
func (d *SelectDialog) Show() {
    currentApp := fyne.CurrentApp()
    if currentApp == nil {
        return
    }

    d.window = currentApp.NewWindow(d.Title)
    d.window.SetContent(d.createUI())
    d.window.Resize(fyne.NewSize(680, 420))
    d.window.Show()
}

// NavigateTo updates CurrentPath, records the previous directory in RecentDirs,
// reloads directory entries from disk if readable, and refreshes UI controls.
func (d *SelectDialog) NavigateTo(targetPath string) {
    if targetPath == "" {
        return
    }

    cleanTarget := filepath.Clean(targetPath)

    d.CurrentPath = cleanTarget
    d.loadDirectoryEntries()
    d.syncLookInSelect()
    d.refreshBrowserViews()
}

// SetViewMode switches the main browser pane between ViewModeList and ViewModeGrid.
func (d *SelectDialog) SetViewMode(mode ViewMode) {
    d.ViewMode = mode
    d.refreshBrowserViews()
}

func (d *SelectDialog) SetPreferredDirs(dirs ...string) {
    d.PreferredDirs = dirs
}

// ancestorPaths returns target followed by each parent directory up to the root.
func (d *SelectDialog) ancestorPaths(target string) []string {
    if target == "" {
        return nil
    }

    clean := filepath.Clean(target)
    var paths []string
    seen := make(map[string]bool)

    for {
        if seen[clean] {
            break
        }
        seen[clean] = true
        paths = append(paths, clean)

        parent := filepath.Dir(clean)
        if parent == "" || parent == clean {
            break
        }
        clean = parent
    }

    if d.PreferredDirs != nil {
        paths = append(paths, d.PreferredDirs...)
    }

    return paths
}

func (d *SelectDialog) syncLookInSelect() {
    if d.lookInSelect == nil || d.CurrentPath == "" {
        return
    }

    clean := filepath.Clean(d.CurrentPath)
    d.lookInSelect.Options = d.ancestorPaths(clean)
    d.lookInSelect.SetSelected(clean)
    d.lookInSelect.Refresh()
}

func (d *SelectDialog) matchesFilter(fileName string) bool {
    if d.filter == nil {
        return true
    }
    fullPath := filepath.Join(d.CurrentPath, fileName)
    return d.filter.Matches(storage.NewFileURI(fullPath))
}

// loadDirectoryEntries populates Items and ItemIsFolder from CurrentPath when
// the directory exists on disk, preserving seeded items otherwise.
func (d *SelectDialog) loadDirectoryEntries() {
    entries, err := os.ReadDir(d.CurrentPath)
    if err != nil {
        return
    }

    folders := make([]string, 0)
    files := make([]string, 0)
    isFolder := make(map[string]bool, len(entries))

    for _, entry := range entries {
        name := entry.Name()
        if !d.ShowHidden && isHiddenName(name) {
            continue
        }
        if entry.IsDir() {
            folders = append(folders, name)
            isFolder[name] = true
        } else if d.matchesFilter(name) {
            files = append(files, name)
            isFolder[name] = false
        }
    }

    sort.Strings(folders)
    sort.Strings(files)

    d.Items = append(folders, files...)
    d.ItemIsFolder = isFolder
}

func (d *SelectDialog) navigateUp() {
    if d.CurrentPath == "" {
        return
    }
    parent := filepath.Dir(filepath.Clean(d.CurrentPath))
    if parent == "" || parent == filepath.Clean(d.CurrentPath) {
        return
    }
    d.NavigateTo(parent)
}

func (d *SelectDialog) navigateHome() {
    d.NavigateTo(xdg.UserHomeDir())
}

func (d *SelectDialog) promptNewFolder() {
    if d.window == nil || d.CurrentPath == "" {
        return
    }

    entryDialog := dialog.NewEntryDialog("New Folder", "Folder name:", func(name string) {
        if name == "" {
            return
        }
        newDirPath := filepath.Join(d.CurrentPath, name)
        if err := os.Mkdir(newDirPath, 0o755); err != nil {
            dialog.ShowError(err, d.window)
            return
        }
        d.loadDirectoryEntries()
        d.refreshBrowserViews()
    }, d.window)
    entryDialog.Show()
}

func (d *SelectDialog) refreshBrowserViews() {
    if d.currentList != nil {
        d.currentList.Refresh()
    }
    if d.currentGrid != nil {
        d.rebuildGrid()
    }
    if d.viewContainer != nil {
        d.viewContainer.Objects = []fyne.CanvasObject{d.activeBrowserObject()}
        d.viewContainer.Refresh()
    }
}

func (d *SelectDialog) activeBrowserObject() fyne.CanvasObject {
    if d.ViewMode == ViewModeGrid && d.currentGrid != nil {
        return container.NewScroll(d.currentGrid)
    }
    return d.currentList
}

func (d *SelectDialog) rebuildGrid() {
    if d.currentGrid == nil {
        return
    }
    objs := make([]fyne.CanvasObject, 0, len(d.Items))
    for _, name := range d.Items {
        itemName := name
        iconRes := theme.DocumentIcon()
        if d.ItemIsFolder[itemName] {
            iconRes = theme.FolderIcon()
        }
        btn := widget.NewButtonWithIcon(itemName, iconRes, func() {
            if d.ItemIsFolder[itemName] {
                d.NavigateTo(filepath.Join(d.CurrentPath, itemName))
            } else if d.fileInput != nil {
                d.fileInput.SetText(itemName)
            }
        })
        btn.Alignment = widget.ButtonAlignCenter
        objs = append(objs, btn)
    }
    d.currentGrid.Objects = objs
    d.currentGrid.Refresh()
}

// createUI assembles the top navigation bar, split directory/history view,
// and bottom input/action footer for the dialog window.
func (d *SelectDialog) createUI() fyne.CanvasObject {
    // Attempt initial filesystem load if CurrentPath exists; keep seeded entries otherwise.
    if d.CurrentPath != "" {
        d.loadDirectoryEntries()
    }

    // 1. Top navigation bar: directory selector label, dropdown, and action icons.
    initialPath := ""
    if d.CurrentPath != "" {
        initialPath = filepath.Clean(d.CurrentPath)
    }
    initialOptions := d.ancestorPaths(initialPath)

    lookInLabel := widget.NewLabelWithStyle("Look In:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
    d.lookInSelect = widget.NewSelect(initialOptions, func(selected string) {
        if selected == "" || d.CurrentPath == "" {
            return
        }
        if filepath.Clean(d.CurrentPath) == filepath.Clean(selected) {
            return
        }
        d.NavigateTo(selected)
    })
    if initialPath != "" {
        d.lookInSelect.SetSelected(initialPath)
    }

    d.upBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() {
        d.navigateUp()
    })
    d.homeBtn = widget.NewButtonWithIcon("", theme.HomeIcon(), func() {
        d.navigateHome()
    })
    d.newFolderBtn = widget.NewButtonWithIcon("", theme.FolderNewIcon(), func() {
        d.promptNewFolder()
    })

    d.miscBtn = NewSplitButtonWithDDD("", func() {},
        fyne.NewMenuItemWithIcon("", theme.GridIcon(), func() {
            d.SetViewMode(ViewModeGrid)
        }),
        fyne.NewMenuItemWithIcon("", theme.ListIcon(), func() {
            d.SetViewMode(ViewModeList)
        }),
        fyne.NewMenuItemWithIcon("", theme.ZoomOutIcon(), func() {
            d.SetShowHidden(false)
        }),
        fyne.NewMenuItemWithIcon("", theme.ZoomInIcon(), func() {
            d.SetShowHidden(true)
        }),
    )

    toolbar := container.NewHBox(d.upBtn, d.homeBtn, d.newFolderBtn, d.miscBtn)
    topBar := container.NewBorder(nil, nil, lookInLabel, toolbar, d.lookInSelect)

    // 2. Center split pane: current directory contents on the left, recents on the right.
    d.currentList = widget.NewList(
        func() int { return len(d.Items) },
        func() fyne.CanvasObject {
            return container.NewHBox(widget.NewIcon(theme.FolderIcon()), widget.NewLabel(""))
        },
        func(id widget.ListItemID, obj fyne.CanvasObject) {
            box := obj.(*fyne.Container)
            icon := box.Objects[0].(*widget.Icon)
            label := box.Objects[1].(*widget.Label)

            name := d.Items[id]
            label.SetText(name)
            if d.ItemIsFolder[name] {
                icon.SetResource(theme.FolderIcon())
            } else {
                icon.SetResource(theme.DocumentIcon())
            }
        },
    )
    d.currentList.OnSelected = func(id widget.ListItemID) {
        if id < 0 || id >= len(d.Items) {
            return
        }
        name := d.Items[id]
        if d.ItemIsFolder[name] && d.CurrentPath != "" {
            d.NavigateTo(filepath.Join(d.CurrentPath, name))
            return
        }
        if d.fileInput != nil {
            d.fileInput.SetText(name)
        }
    }

    d.currentGrid = container.NewGridWrap(fyne.NewSize(150, 44))
    d.rebuildGrid()

    d.viewContainer = container.NewMax(d.activeBrowserObject())

    // 3. Bottom footer: file name entry, file type filter, and Open/Cancel actions.
    nameLabel := widget.NewLabelWithStyle("File Name:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

    typeLabel := widget.NewLabelWithStyle("Files of Type:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
    initialLabels := make([]string, len(d.fileTypeOptions))
    for i, opt := range d.fileTypeOptions {
        initialLabels[i] = opt.Label
    }

    d.typeSelect = widget.NewSelect(initialLabels, func(selected string) {
        for _, opt := range d.fileTypeOptions {
            if opt.Label == selected {
                d.filter = opt.Filter
                if d.CurrentPath != "" {
                    d.loadDirectoryEntries()
                }
                d.refreshBrowserViews()
                return
            }
        }
    })
    if len(initialLabels) > 0 {
        d.typeSelect.SetSelected(initialLabels[0])
    }

    inputs := container.NewGridWithRows(2,
        container.NewBorder(nil, nil, nameLabel, nil, d.fileInput),
        container.NewBorder(nil, nil, typeLabel, nil, d.typeSelect),
    )

    cancelBtn := widget.NewButton("Cancel", func() {
        if d.OnCancelled != nil {
            d.OnCancelled()
        }
        if d.window != nil {
            d.window.Close()
        }
    })

    buttons := container.NewVBox(d.openBtn, cancelBtn)
    footer := container.NewBorder(nil, nil, nil, buttons, inputs)

    // 4. Outer padding around the main layout regions.
    return container.NewPadded(
        container.NewBorder(topBar, footer, nil, nil, d.viewContainer),
    )
}

// CreateRenderer returns a minimal renderer since Show manages the dialog window.
func (d *SelectDialog) CreateRenderer() fyne.WidgetRenderer {
    return widget.NewSimpleRenderer(container.NewMax())
}

func (d *SelectDialog) SetFileName(f string) {
    d.fileInput.SetText(f)
}

func (d *SelectDialog) SetAction(f string) {
    d.openBtn.SetText(f)
}
