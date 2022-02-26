document.getElementById("logout")
  .addEventListener("click", () => {
    fetch("/logout", {
      method: "POST",
    })
      .then(() => {
        window.location.assign("/")
      })
  })

document
  .querySelectorAll("button[data-delete-id]")
  .forEach(button => {
    const id = button.dataset.deleteId

    button.addEventListener("click", () => {
      fetch("/alert/" + id, {
        method: "DELETE",
      })
        .then(response => {
          if (response.ok) {
            window.location.reload()
          }
        })
    })
  })

// Format the action of forms set up that way.
document
  .querySelectorAll("form[data-format-action]")
  .forEach(form => {
    const actionTemplate = form.action
    const actionVariables = {}

    form.querySelectorAll("[data-format-action]")
      .forEach(elem => {
        const replacePlaceholder = () => {
          actionVariables[elem.dataset.formatAction] = elem.value
          let action = actionTemplate

          Object.entries(actionVariables)
            .forEach(([key, value]) => {
              action = action
                .replace('{' + key + '}', value)
                .replace('%7B' + key + '%7D', value)
            })

          form.action = action
        }

        replacePlaceholder()

        elem.addEventListener("change", () => {
          replacePlaceholder()
        })

        elem.addEventListener("click", () => {
          replacePlaceholder()
        })

        elem.addEventListener("submit", () => {
          replacePlaceholder()
        })
      })
  })
