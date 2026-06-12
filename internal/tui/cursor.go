package tui

// moveFileCursor changes the selected file and switches the viewer to preview mode.
func (m Model) moveFileCursor(delta int) Model {
	if len(m.files) == 0 {
		m.notice = "No changed files"
		return m
	}
	m.fileCursor = clamp(m.fileCursor+delta, 0, len(m.files)-1)
	m.target = m.files[m.fileCursor].Path
	m.mode = "preview"
	m.err = nil
	m.notice = ""
	m.ensureFileCursorVisible()
	return m
}

// moveBranchCursor moves through the current branch picker rows.
func (m Model) moveBranchCursor(delta int) Model {
	if len(m.branches) == 0 {
		m.notice = "No branches found"
		return m
	}
	m.branchCursor = clamp(m.branchCursor+delta, 0, len(m.branches)-1)
	m.ensureBranchCursorVisible()
	return m
}

// moveProfileCursor moves through configured identity profiles.
func (m Model) moveProfileCursor(delta int) Model {
	if len(m.config.Profiles) == 0 {
		m.notice = "No profiles configured"
		return m
	}
	m.profileCursor = clamp(m.profileCursor+delta, 0, len(m.config.Profiles)-1)
	m.ensureProfileCursorVisible()
	return m
}

// reconcileFileCursor keeps the selected file valid after the repo refreshes.
func (m *Model) reconcileFileCursor() {
	if len(m.files) == 0 {
		m.fileCursor = 0
		m.fileOffset = 0
		if m.mode == "preview" {
			m.target = "repo"
			m.mode = "review"
		}
		return
	}
	if m.target != "" && m.target != "repo" {
		for i, file := range m.files {
			if file.Path == m.target {
				m.fileCursor = i
				m.ensureFileCursorVisible()
				return
			}
		}
	}
	m.fileCursor = clamp(m.fileCursor, 0, len(m.files)-1)
	m.ensureFileCursorVisible()
}

// reconcileBranchCursor keeps the current branch selected after branches load.
func (m *Model) reconcileBranchCursor() {
	if len(m.branches) == 0 {
		m.branchCursor = 0
		m.branchOffset = 0
		return
	}
	for i, branch := range m.branches {
		if branch == m.info.Branch {
			m.branchCursor = i
			break
		}
	}
	m.branchCursor = clamp(m.branchCursor, 0, len(m.branches)-1)
	m.ensureBranchCursorVisible()
}

// reconcileProfileCursor starts the profile picker near the active Git identity.
func (m *Model) reconcileProfileCursor() {
	profiles := m.config.Profiles
	if len(profiles) == 0 {
		m.profileCursor = 0
		m.profileOffset = 0
		return
	}
	for i, profile := range profiles {
		if profile.Name == m.info.User || profile.Email == m.info.User {
			m.profileCursor = i
			break
		}
	}
	m.profileCursor = clamp(m.profileCursor, 0, len(profiles)-1)
	m.ensureProfileCursorVisible()
}

// ensureFileCursorVisible scrolls the files list so the selected file is visible.
func (m *Model) ensureFileCursorVisible() {
	visibleRows := max(1, max(8, m.height-8)-4)
	if m.fileCursor < m.fileOffset {
		m.fileOffset = m.fileCursor
	}
	if m.fileCursor >= m.fileOffset+visibleRows {
		m.fileOffset = m.fileCursor - visibleRows + 1
	}
	m.fileOffset = clamp(m.fileOffset, 0, max(0, len(m.files)-visibleRows))
}

// ensureBranchCursorVisible scrolls the branch picker so the selected branch is visible.
func (m *Model) ensureBranchCursorVisible() {
	visibleRows := max(1, max(8, m.height-8)-4)
	if m.branchCursor < m.branchOffset {
		m.branchOffset = m.branchCursor
	}
	if m.branchCursor >= m.branchOffset+visibleRows {
		m.branchOffset = m.branchCursor - visibleRows + 1
	}
	m.branchOffset = clamp(m.branchOffset, 0, max(0, len(m.branches)-visibleRows))
}

// ensureProfileCursorVisible scrolls the profile picker so the selected profile is visible.
func (m *Model) ensureProfileCursorVisible() {
	visibleRows := max(1, max(8, m.height-8)-4)
	if m.profileCursor < m.profileOffset {
		m.profileOffset = m.profileCursor
	}
	if m.profileCursor >= m.profileOffset+visibleRows {
		m.profileOffset = m.profileCursor - visibleRows + 1
	}
	m.profileOffset = clamp(m.profileOffset, 0, max(0, len(m.config.Profiles)-visibleRows))
}
