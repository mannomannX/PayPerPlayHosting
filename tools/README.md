# Version Features Updater Tool

This directory contains maintenance tools for managing the version-features compatibility data.

## update-version-features.go

A CLI tool to help maintain the `web/static/js/version-features.js` file.

### Features

1. **Add New Feature Mappings** - Interactively add new Minecraft features and their minimum versions
2. **View Current Mappings** - Display all currently configured feature-version mappings
3. **Export to JSON** - Create JSON backups of the feature data
4. **Check Minecraft Wiki** - Quick lookup tool to find Minecraft Wiki pages for features

### Usage

From the project root directory:

```bash
# Compile and run
go run tools/update-version-features.go

# Or compile once and run multiple times
go build -o tools/version-updater tools/update-version-features.go
./tools/version-updater
```

### Workflow for Adding New Features

1. Run the tool: `go run tools/update-version-features.go`
2. Select option 1 (Add new feature mapping)
3. Enter the feature details:
   - Feature name: e.g., `gamemode_spectator`
   - Minimum version: e.g., `1.8.0`
   - Source: e.g., `Minecraft Wiki`
   - Notes: e.g., `Added in 1.8 snapshot 14w05a`
4. The tool will show you the exact line to add to `version-features.js`
5. Manually update `web/static/js/version-features.js` with the new line
6. Test the changes in the UI

### When to Use This Tool

Use this tool when:
- Adding support for new Minecraft features (Phase 2, Phase 3, etc.)
- Updating version requirements for existing features
- Creating backups of the feature data
- Researching Minecraft version history

### Important Notes

- This tool does NOT automatically modify `version-features.js`
- All changes must be manually reviewed and applied
- Always test changes in the UI after updating the file
- Create JSON backups before making major changes
- Verify version information from official sources (Minecraft Wiki, Mojang docs)

### Example: Adding a New Feature

Let's say we want to add support for "Bundles" (1.21+):

1. Run the tool
2. Select "Add new feature mapping"
3. Enter:
   - Feature: `bundle_item`
   - Min Version: `1.21.0`
   - Source: `Minecraft Wiki`
   - Notes: `Bundles added in 1.21`
4. The tool outputs:
   ```javascript
   'bundle_item': '1.21.0',  // Bundles added in 1.21
   ```
5. Add this line to `FEATURE_MIN_VERSIONS` in `version-features.js`
6. Update frontend logic to check for bundle support

## Future Enhancements

Potential improvements for this tool:
- Automatic parsing of Minecraft Wiki pages
- Integration with Mojang's version manifest API
- Automatic JS file updates with backup creation
- Validation of semantic version strings
- Bulk import from JSON files
