package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type CategoryRowVM struct {
	ID   string
	Name string
	Type string
	CSRF string
}

func categoryRowFromModel(cat categories.Category, csrf string) CategoryRowVM {
	return CategoryRowVM{ID: cat.ID, Name: cat.Name, Type: string(cat.Type), CSRF: csrf}
}

func normalizeCategoryType(s string) categories.Type {
	return categories.Type(strings.TrimSpace(strings.ToLower(s)))
}

func isValidCategoryType(t categories.Type) bool {
	return t == categories.TypeIncome || t == categories.TypeExpense
}

func (h *Handlers) GetCategoriesPage(c *gin.Context) {
	if h.Categories == nil {
		c.String(http.StatusInternalServerError, "categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	items, err := h.Categories.List(c.Request.Context(), wsID)
	if err != nil {
		c.String(http.StatusInternalServerError, "could not list categories")
		return
	}

	csrf := GenerateCSRF(h.CSRFSecret, h.CSRFTTL)

	rows := make([]CategoryRowVM, 0, len(items))
	for _, it := range items {
		rows = append(rows, categoryRowFromModel(it, csrf))
	}

	h.render(c, "app/categories.html", gin.H{
		"Title":      "Categories",
		"BodyClass":  "app-dark app-solid",
		"MainClass":  "tx-main",
		"Workspace":  workspaceFromContext(c),
		"CSRF":       csrf,
		"Categories": rows,
		"Flash":      c.Query("flash"),
	})
}

func (h *Handlers) PostCreateCategory(c *gin.Context) {
	if h.Categories == nil {
		c.String(http.StatusInternalServerError, "categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	name := c.PostForm("name")
	t := normalizeCategoryType(c.PostForm("type"))
	if !isValidCategoryType(t) {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Type must be income or expense"}})
		return
	}

	cat, err := h.Categories.Create(c.Request.Context(), wsID, name, t)
	if err != nil {
		switch {
		case errors.Is(err, categories.ErrInvalidName):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Name must be 1..60 characters"}})
		case errors.Is(err, categories.ErrInvalidType):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Type must be income or expense"}})
		case errors.Is(err, categories.ErrCategoryExists):
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category already exists"}})
		default:
			c.String(http.StatusInternalServerError, "could not create category")
		}
		return
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "cat_clear_errors", gin.H{})
	_, _ = c.Writer.WriteString("\n<table style=\"display:none\"><tbody>\n<tr id=\"cat-empty\" hx-swap-oob=\"delete\"></tr>\n</tbody></table>\n")
	_, _ = c.Writer.WriteString("\n<table id=\"cat-fragment\"><tbody>\n")
	h.renderPartial(c, "cat_row", gin.H{
		"ID":   cat.ID,
		"Name": cat.Name,
		"Type": string(cat.Type),
	})
	_, _ = c.Writer.WriteString("\n</tbody></table>\n")
}

func (h *Handlers) PostUpdateCategory(c *gin.Context) {
	if h.Categories == nil {
		c.String(http.StatusInternalServerError, "categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	catID := strings.TrimSpace(c.Param("id"))
	if catID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	name := c.PostForm("name")
	t := normalizeCategoryType(c.PostForm("type"))
	if !isValidCategoryType(t) {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Type must be income or expense"}})
		h.renderPartial(c, "cat_row", gin.H{"ID": catID, "Name": strings.TrimSpace(name), "Type": string(t)})
		return
	}

	out, err := h.Categories.Update(c.Request.Context(), wsID, catID, name, t)
	if err != nil {
		switch {
		case errors.Is(err, categories.ErrInvalidName):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Name must be 1..60 characters"}})
		case errors.Is(err, categories.ErrInvalidType):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Type must be income or expense"}})
		case errors.Is(err, categories.ErrCategoryExists):
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category already exists"}})
		case errors.Is(err, categories.ErrCategoryNotFound):
			c.Status(http.StatusNotFound)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category not found"}})
		default:
			c.String(http.StatusInternalServerError, "could not update category")
			return
		}
		h.renderPartial(c, "cat_row", gin.H{"ID": catID, "Name": strings.TrimSpace(name), "Type": string(t)})
		return
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "cat_clear_errors", gin.H{})
	h.renderPartial(c, "cat_row", gin.H{"ID": out.ID, "Name": out.Name, "Type": string(out.Type)})
}

func (h *Handlers) PostDeleteCategory(c *gin.Context) {
	if h.Categories == nil {
		c.String(http.StatusInternalServerError, "categories service is not configured")
		return
	}

	wsID := c.GetString(workspaces.CtxWorkspaceIDKey)
	if wsID == "" {
		c.String(http.StatusInternalServerError, "workspace not set")
		return
	}

	catID := strings.TrimSpace(c.Param("id"))
	if catID == "" {
		c.String(http.StatusBadRequest, "missing id")
		return
	}

	var reassignPtr *string
	if v := strings.TrimSpace(c.PostForm("reassign_to")); v != "" {
		reassignPtr = &v
	}

	err := h.Categories.Delete(c.Request.Context(), wsID, catID, reassignPtr)
	if err != nil {
		switch {
		case errors.Is(err, categories.ErrCategoryInUse) && reassignPtr == nil:
			items, lerr := h.Categories.List(c.Request.Context(), wsID)
			if lerr != nil {
				c.String(http.StatusInternalServerError, "could not list categories")
				return
			}
			others := make([]CategoryRowVM, 0, len(items))
			for _, it := range items {
				if it.ID == catID {
					continue
				}
				others = append(others, categoryRowFromModel(it, ""))
			}

			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_clear_errors", gin.H{})
			h.renderPartial(c, "cat_delete_reassign", gin.H{
				"ID":              catID,
				"OtherCategories": others,
			})
			return
		case errors.Is(err, categories.ErrCategoryNotFound):
			c.Status(http.StatusNotFound)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category not found"}})
			return
		case errors.Is(err, categories.ErrCategoryInUse):
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category is used by transactions"}})
			return
		default:
			c.String(http.StatusInternalServerError, "could not delete category")
			return
		}
	}

	c.Status(http.StatusOK)
	h.renderPartial(c, "cat_clear_errors", gin.H{})
	_, _ = c.Writer.WriteString("\n<tr id=\"cat-" + catID + "\" hx-swap-oob=\"delete\"></tr>\n")

	items, lerr := h.Categories.List(c.Request.Context(), wsID)
	if lerr == nil && len(items) == 0 {
		_, _ = c.Writer.WriteString("\n<tr id=\"cat-empty\" hx-swap-oob=\"beforeend:#cat-tbody\"><td colspan=\"3\">No categories</td></tr>\n")
	}
}
