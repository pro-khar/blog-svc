const response = await fetch("http://localhost:8080/admin/posts/15/un-publish",{
    method: "POST",
    headers: {
        "Content-Type":"application/json"
    },
    // body:JSON.stringify({
    //     "title": "Andi mandi sandi",
    // "slug": "hello-world",
    // "excerpt": "nigga please",
    // "content": "Lorem ipsum dolor",
    // "tags": [
    //   "go",
    //   "blog",
    //   "ts"
    // ],
    // })
})

if(response.status==200){
    console.log(await response.json())
}else{
    console.error(response.status, await response.text())
}
export {}                                   