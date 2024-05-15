<div class="p-3 row">

    <h3>Glob Groups</h3>
<div class="card">
  <div class="card-body">
    Globs are like simplified wildcards, that can be used to match a variety of urls.
  </div>
</div>
    <table class="table" id="globs">
        <thead>
            <tr>
                <th scope="col">#</th>
                <th scope="col">Name</th>
                <th scope="col">Description</th>
                <th scope="col">Device</th>
                <th scope="col">Glob</th>
            </tr>
        </thead>
        <tbody>
            {{ range .Globs }}
            <tr>
                <th scope="row"></th>
                <td>{{ .Name }}</td>
                <td>{{ .Description }}</td>
                <td>{{ .Device }}</td>
                <td>{{ .Glob }}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
            
            <h4>Add new glob</h4>

    <form action="/globs/add" method="POST" class="p-3">
        <div class="row">

            <div class="col">

                <div class="form-floating mb-3">
                    <input type="text" class="form-control" placeholder="" id="name" name="name" />
                    <label for="name" class="form-label">Name</label>
                </div>
            </div>
            <div class="col">
                <div class="form-floating mb-3">
                    <input type="text" class="form-control" placeholder="" id="description" name="description" />
                    <label for="description" class="form-label">Description</label>
                </div>
            </div>

            <div class="col">
                <div class="form-floating mb-3">
                    <select class="form-select" id="device">
                        <option selected>Device</option>
                        <option value="1">Laptop</option>
                        <option value="2">Phone 1</option>
                        <option value="3">Phone 2</option>
                    </select>
                    <label for="device" class="form-label">Device</label>
                </div>
            </div>

            <div class="col">
                <div class="form-floating mb-3">
                    <input type="text" class="form-control" placeholder="*.instagram.com" id="glob" name="glob" />
                    <label for="glob" class="form-label">Glob</label>
                </div>
            </div>

        </div>

        <div class="input-group mb-3">
            <button class="btn btn-primary" type="submit" id="add_user"> Add Glob
            </button>
        </div>
    </form>
</div>