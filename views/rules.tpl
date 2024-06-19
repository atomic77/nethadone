<div class="p-3 row">

    <h3>Glob Matches</h3>

    <table class="table" id="glob-usage">
        <thead>
            <tr>
                <th scope="col">#</th>
                <th scope="col">SrcIp</th>
                <th scope="col">DestIp</th>
                <th scope="col">Likely Domain</th>
                <th scope="col">Glob Group</th>
                <th scope="col">Bytes</th>
            </tr>
        </thead>
        <tbody>
            {{ range .BandwidthList }}
            <tr>
                <th scope="row"></th>
                <td>{{ .SrcIpAddr }}</td>
                <td>{{ .DestIpAddr }}</td>
                <td>{{ .ProbDomain }}</td>
                <td>{{ .GlobName }}</td>
                <td>{{ .Bytes }}</td>
            </tr>
            {{end}}

        </tbody>
    </table>

    <form action="/rulesets/change" method="POST" class="p-3">
        <div class="row">

            <div class="col">

                <div class="form-floating mb-3">
                    <input type="text" class="form-control" placeholder="192,168,0,14" id="src" name="src" />
                    <label for="src" class="form-label">Source IP</label>
                </div>
            </div>
            <div class="col">
                <div class="form-floating mb-3">
                    <input type="text" class="form-control" placeholder="192,168,0,108" id="dest" name="dest" />
                    <label for="dest" class="form-label">Destination IP</label>
                </div>
            </div>

            <div class="col">
                <div class="form-floating mb-3">
                    <input type="number" class="form-control" value="500"  id="delay" name="delay" />
                    <label for="delay" class="form-label">Delay (ms)</label>
                </div>
            </div>
        </div>

        <div class="input-group mb-3">
            <button class="btn btn-primary" type="submit" id="add_user"> Change Rule
            </button>
        </div>
    </form>
</div>