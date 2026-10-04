const response = await fetch("http://localhost:8080/admin/posts/10",{
    method: "DELETE",
    headers: {
        "Content-Type":"application/json"
    }
    // body:JSON.stringify({
    //     "title": "Andi mandi sandi",
    // "slug": "hello-world",
    // "excerpt": "My very first post on this blog.",
    // "content": "Lorem ipsum dolor",
    // "tags": [
    //   "go",
    //   "blog",
    //   "first-post"
    // ],
    // })
})

if(response.status==204){
    console.log(await response.json())
}else{
    console.error(response.status, await response.text())
}
export {}