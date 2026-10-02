const response = await fetch("http://localhost:8080/posts",{
    method: "POST",
    headers: {
        "Content-Type":"application/json"
    },
    body:JSON.stringify({
        title:" ",
        slug:"test-data",
        excerpt: "this is an example summary",
        content:"###Again, This post was sent as a POST request from a client",
        tags:["lorem", "ipsum", "dolor", "siet"], 
        is_published:true
    })
})

if(response.status==201){
    console.log(await response.json())
}else{
    console.error(response.status, await response.text())
}
export {}