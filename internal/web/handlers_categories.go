package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/skelbigo/FinanceTracker/internal/categories"
	"github.com/skelbigo/FinanceTracker/internal/workspaces"
)

type catRowVM struct {
	ID   string
	Name string
	Type string
}

func catRowFromModel(c categories.Category) catRowVM {
	return catRowVM{ID: c.ID, Name: c.Name, Type: string(c.Type)}
}

func normalizeCatType(s string) categories.Type {
	return categories.Type(strings.TrimSpace(strings.ToLower(s)))
}

func validateCatType(t categories.Type) bool {
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

	rows := make([]catRowVM, 0, len(items))
	for _, it := range items {
		rows = append(rows, catRowFromModel(it))
	}

	h.render(c, "app/categories.html", gin.H{
		"Title":      "Categories",
		"BodyClass":  "app-dark app-solid",
		"MainClass":  "tx-main",
		"Workspace":  workspaceFromContext(c),
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
	t := normalizeCatType(c.PostForm("type"))
	if !validateCatType(t) {
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

	row := catRowFromModel(cat)
	h.renderPartial(c, "cat_create_response", gin.H{"Row": row})
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
	t := normalizeCatType(c.PostForm("type"))
	if !validateCatType(t) {
		c.Status(http.StatusUnprocessableEntity)
		h.renderPartial(c, "cat_update_error", gin.H{
			"Errors": []string{"Type must be income or expense"},
			"Row":    catRowVM{ID: catID, Name: strings.TrimSpace(name), Type: string(t)},
		})
		return
	}

	out, err := h.Categories.Update(c.Request.Context(), wsID, catID, name, t)
	if err != nil {
		switch {
		case errors.Is(err, categories.ErrInvalidName):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_update_error", gin.H{
				"Errors": []string{"Name must be 1..60 characters"},
				"Row":    catRowVM{ID: catID, Name: strings.TrimSpace(name), Type: string(t)},
			})
		case errors.Is(err, categories.ErrInvalidType):
			c.Status(http.StatusUnprocessableEntity)
			h.renderPartial(c, "cat_update_error", gin.H{
				"Errors": []string{"Type must be income or expense"},
				"Row":    catRowVM{ID: catID, Name: strings.TrimSpace(name), Type: string(t)},
			})
		case errors.Is(err, categories.ErrCategoryExists):
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_update_error", gin.H{
				"Errors": []string{"Category already exists"},
				"Row":    catRowVM{ID: catID, Name: strings.TrimSpace(name), Type: string(t)},
			})
		case errors.Is(err, categories.ErrCategoryNotFound):
			c.Status(http.StatusNotFound)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category not found"}})
		default:
			c.String(http.StatusInternalServerError, "could not update category")
		}
		return
	}

	row := catRowFromModel(out)
	h.renderPartial(c, "cat_update_response", gin.H{"Row": row})
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
			others := make([]catRowVM, 0, len(items))
			for _, it := range items {
				if it.ID == catID {
					continue
				}
				others = append(others, catRowFromModel(it))
			}
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_delete_reassign_response", gin.H{
				"ID":              catID,
				"OtherCategories": others,
			})
			return
		case errors.Is(err, categories.ErrCategoryNotFound):
			c.Status(http.StatusNotFound)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category not found"}})
			return
		case errors.Is(err, categories.ErrCategoryInUse):
			// Reassign flow failed to clear usage (shouldn't usually happen)
			c.Status(http.StatusConflict)
			h.renderPartial(c, "cat_errors", gin.H{"Errors": []string{"Category is used by transactions"}})
			return
		default:
			c.String(http.StatusInternalServerError, "could not delete category")
			return
		}
	}

	h.renderPartial(c, "noop", gin.H{})
}
