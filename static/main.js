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
