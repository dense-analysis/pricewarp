// Modal cancel buttons.
document
  .querySelectorAll(".modal [data-cancel]")
  .forEach(button => {
    button.addEventListener("click", () => {
      const modal = button.closest('.modal')

      if (modal) {
        modal.setAttribute('hidden', '')
      }
    })
  })

// The logout button logging out via POST.
document.getElementById("logout")
  .addEventListener("click", () => {
    fetch("/logout", {
      method: "POST",
    })
      .then(() => {
        window.location.assign("/")
      })
  })

// Opening a modal to confirm deleting an alert.
document
  .querySelectorAll("button[data-try-delete-id]")
  .forEach(button => {
    button.addEventListener("click", () => {
      document
        .querySelectorAll("[data-confirm-delete-modal] [data-confirm]")
        .forEach(confirmButton => {
          confirmButton.dataset.deleteId = button.dataset.tryDeleteId
        })

      document
        .querySelectorAll("[data-confirm-delete-modal]")
        .forEach(modal => {
          modal.removeAttribute('hidden')
        })
    })
  })

// Confirmation of deleting an alert.
document
  .querySelectorAll("[data-confirm-delete-modal] [data-confirm]")
  .forEach(button => {
    button.addEventListener("click", () => {
      fetch("/alert/" + button.dataset.deleteId, {
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
