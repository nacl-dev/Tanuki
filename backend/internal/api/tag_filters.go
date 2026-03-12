package api

// tagExpressionMatchSQL matches either a legacy raw tag name or a supported
// namespace:value expression against tags.name and tags.category.
func tagExpressionMatchSQL(tagExpr, nameExpr, categoryExpr string) string {
	namespaceExpr := "LOWER(split_part(" + tagExpr + ", ':', 1))"
	valueExpr := "LOWER(split_part(" + tagExpr + ", ':', 2))"

	return `(
		LOWER(` + nameExpr + `) = LOWER(` + tagExpr + `)
		OR EXISTS (
			SELECT 1
			FROM tag_aliases ta
			WHERE ta.tag_id = t.id
			  AND LOWER(ta.alias_name) = LOWER(` + tagExpr + `)
		)
		OR (
			POSITION(':' IN ` + tagExpr + `) > 0
			AND ` + valueExpr + ` <> ''
			AND LOWER(` + nameExpr + `) = ` + valueExpr + `
			AND (
				(` + namespaceExpr + ` IN ('general', 'tag', 'tags') AND ` + categoryExpr + ` = 'general')
				OR (` + namespaceExpr + ` IN ('artist', 'artists', 'author', 'authors', 'creator', 'creators', 'circle', 'circles', 'group', 'groups') AND ` + categoryExpr + ` = 'artist')
				OR (` + namespaceExpr + ` IN ('character', 'characters', 'char') AND ` + categoryExpr + ` = 'character')
				OR (` + namespaceExpr + ` IN ('parody', 'parodies', 'copyright', 'copyrights', 'series', 'franchise', 'property') AND ` + categoryExpr + ` = 'parody')
				OR (` + namespaceExpr + ` IN ('genre', 'genres', 'male', 'female', 'mixed', 'other', 'species', 'theme', 'themes', 'fetish', 'fetishes', 'category', 'categories', 'format', 'formats') AND ` + categoryExpr + ` = 'genre')
				OR (` + namespaceExpr + ` IN ('meta', 'title', 'page', 'rating', 'language', 'lang', 'source', 'site', 'uploader', 'date') AND ` + categoryExpr + ` = 'meta')
			)
		)
	)`
}
