// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.857
package ui

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

func Error() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<!doctype html><html lang=\"de\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"><title>Fehler</title><style>\r\n            body {\r\n                font-family: Arial, sans-serif;\r\n                background-color: #f8d7da;\r\n                color: #721c24;\r\n                display: flex;\r\n                justify-content: center;\r\n                align-items: center;\r\n                height: 100vh;\r\n                margin: 0;\r\n            }\r\n            .error-container {\r\n                text-align: center;\r\n                padding: 20px;\r\n                border: 1px solid #f5c6cb;\r\n                background-color: #f8d7da;\r\n                border-radius: 5px;\r\n            }\r\n            .error-container h1 {\r\n                font-size: 2em;\r\n                margin: 0;\r\n            }\r\n            .error-container p {\r\n                margin: 10px 0 0;\r\n            }\r\n        </style></head><body><div class=\"error-container\"><h1>Ein Fehler ist aufgetreten</h1><p>Entschuldigung, es gab ein Problem bei der Verarbeitung Ihrer Anfrage.</p></div></body></html>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
