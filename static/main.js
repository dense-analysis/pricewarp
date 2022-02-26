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

    form.querySelectorAll("[data-format-action]")
      .forEach(elem => {
        const replacePlaceholder = () => {
          const replaceName = elem.dataset.formatAction

          form.action = actionTemplate
            .replace('{' + replaceName + '}', elem.value)
            .replace('%7B' + replaceName + '%7D', elem.value)
        }

        replacePlaceholder()

        elem.addEventListener("change", () => {
          replacePlaceholder()
        })
      })
  })
