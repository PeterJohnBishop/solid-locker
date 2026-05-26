# solid-locker


## about

A context-aware TUI served over SSH providing server side encrypted upload/download to a SqLite database, and client side TUI to select and download files via SSH stream.

Concurrently, a Gin server REST API provides upload and download endpoints for encrypted streamed upload and download.

![screenshot1](https://github.com/PeterJohnBishop/solid-locker/blob/main/assets/screen1.png?raw=true)

From the localhost machine, the option to pick a file to upload to the file vault is availible. 
On upload, the file is immediately chucked, and the chunk data is encrypted as it's sent to the sqLite database.

![screenshot2](https://github.com/PeterJohnBishop/solid-locker/blob/main/assets/screen2.png?raw=true)

The encrypted file is then listed in the vault contents. 

![screenshot3](https://github.com/PeterJohnBishop/solid-locker/blob/main/assets/screen3.png?raw=true)

And can be seen from the client side, and is availible for download. 
Pressing 'd' automatically generates the ssh command to download the file, and adds it to your clipboard. 


